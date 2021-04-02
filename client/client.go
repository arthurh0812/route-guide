package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/arthurh0812/routeguide/data"
	"github.com/arthurh0812/routeguide/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"log"
	"math/rand"
	"time"
)

var (
	tls                = flag.Bool("tls", false, "Connection uses TLS if set to true, otherwise plain TCP")
	caFile             = flag.String("ca_file", "", "The file containing CA root cert file")
	serverAddr         = flag.String("server_addr", "localhost:8080", "The server address to communicate to")
	serverHostOverride = flag.String("server_host_override", "x.test.youtube.com", "The server name"+
		"used to verify the hostname returned by the TLS handshake")
)

func main() {
	var opts []grpc.DialOption

	if *tls {
		if *caFile == "" {
			*caFile = data.Path("x509/ca_cert.pem")
		}
		creds, err := credentials.NewClientTLSFromFile(*caFile, *serverHostOverride)
		if err != nil {
			log.Fatalf("failed to generate credentials for TLS: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("failed to establish TCP connection: %v", err)
	}
	defer conn.Close()
	client := pb.NewRouteGuideClient(conn)

	feature, err := receiveFeature(client, &pb.Point{Latitude: 409146138, Longitude: -746188906})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("success: got feature: %v", feature)

	feature, err = receiveFeature(client, &pb.Point{Latitude: 0, Longitude: 0})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("success: got feature: %v", feature)

	features, err := receiveFeaturesList(client, &pb.Area{
		TopLeft:     &pb.Point{Latitude: 420000000, Longitude: -730000000},
		BottomRight: &pb.Point{Latitude: 400000000, Longitude: -750000000},
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range features {
		log.Printf("name: %s\n", f.GetName())
		log.Printf("point: (%d, %d)", f.GetLocation().GetLatitude(), f.GetLocation().GetLongitude())
	}

	summary, err := createRoute(client)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Route summary: %v\n", summary)

	routeNotes, err := routeChat(client)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("reading route notes:")
	for _, note := range routeNotes {
		log.Printf("route message: %s, point(%d, %d)\n",
			note.Message, note.Location.Latitude, note.Location.Longitude)
	}
}

// receiveFeature transmits a message to the gRPC RouteGuide server containing the provided point as the location.
// receiveFeature requests the Feature that is located at this location, and if none such Feature could be found, it
// it return nothing
func receiveFeature(client pb.RouteGuideClient, point *pb.Point) (*pb.Feature, error) {
	log.Printf("trying to access a feature with the location (%d, %d)", point.Latitude, point.Longitude)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	feature, err := client.GetFeature(ctx, point)
	if err != nil {
		return nil, fmt.Errorf("failed to call RFC `GetFeatures`: %v", err)
	}
	return feature, nil
}

// receiveFeaturesList transmits a message to the gRPC RouteGuide server containing the provided area as the constraint
// for the area in which the features have to be. It returns a
func receiveFeaturesList(client pb.RouteGuideClient, area *pb.Area) ([]*pb.Feature, error) {
	log.Printf("trying to access all features within %v", area)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	stream, err := client.ListFeatures(ctx, area)
	if err != nil {
		return nil, fmt.Errorf("failed to call RFC `ListFeatures`: %v", err)
	}
	features := make([]*pb.Feature, 0)
	for {
		feature, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to receive feature from stream: %v", err)
		}
		features = append(features, feature)
	}
	return features, nil
}

func createRoute(client pb.RouteGuideClient) (*pb.RouteSummary, error) {
	// generate a random seed
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	pointCount := int(r.Int31n(100) + 2) // the random count of points has to be at least 2
	var points = make([]*pb.Point, 0, pointCount)
	for i := 0; i < pointCount; i++ {
		points = append(points, randomPoint(r))
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second+10)
	defer cancel()
	stream, err := client.CreateRoute(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call RFC `CreateRoute`: %v", err)
	}
	for _, point := range points {
		if err := stream.Send(point); err != nil {
			return nil, fmt.Errorf("failed to send the point: %v", err)
		}
	}

	summary, err := stream.CloseAndRecv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive route summary: %v", err)
	}
	return summary, nil
}

func randomPoint(r *rand.Rand) *pb.Point {
	lat := (r.Int31n(180) - 90) * 1e7
	long := (r.Int31n(360) - 180) * 1e7
	return &pb.Point{Latitude: lat, Longitude: long}
}

func routeChat(client pb.RouteGuideClient) (routeNotes []*pb.RouteNote, err error) {
	notes := []*pb.RouteNote{
		{Location: &pb.Point{Latitude: 0, Longitude: 1}, Message: "First message"},
		{Location: &pb.Point{Latitude: 0, Longitude: 2}, Message: "Second message"},
		{Location: &pb.Point{Latitude: 0, Longitude: 3}, Message: "Third message"},
		{Location: &pb.Point{Latitude: 0, Longitude: 1}, Message: "Fourth message"},
		{Location: &pb.Point{Latitude: 0, Longitude: 2}, Message: "Fifth message"},
		{Location: &pb.Point{Latitude: 0, Longitude: 3}, Message: "Sixth message"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	stream, err := client.RouteChat(ctx)
	if err != nil {
		log.Fatalf("failed to call RFC `RouteChat`: %v", err)
	}
	responses := make(chan *pb.RouteNote)
	errors := make(chan error)
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				close(responses)
				return
			}
			if err != nil {
				errors <- fmt.Errorf("failed to receive route note from stream: %v", err)
				return
			}
			responses <- in
		}
	}()

	for _, note := range notes {
		if err := stream.Send(note); err != nil {
			return nil, fmt.Errorf("failed to send route note to stream: %v", err)
		}
	}
	err = stream.CloseSend()
	if err != nil {
		return nil, err
	}

	for {
		select {
		case err := <-errors:
			return nil, err
		case note, ok := <-responses:
			if !ok {
				return
			}
			routeNotes = append(routeNotes, note)
		}
	}
}
