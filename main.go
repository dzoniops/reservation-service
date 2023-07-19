package main

import (
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/dzoniops/common/pkg/reservation"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"

	"github.com/dzoniops/reservation-service/db"
	"github.com/dzoniops/reservation-service/services"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	db.InitDB()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", os.Getenv("PORT")))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterReservationServiceServer(s, &services.Server{})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
