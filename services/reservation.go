package services

import (
	"context"

	pb "github.com/dzoniops/common/pkg/reservation"

	_ "github.com/dzoniops/reservation-service/db"
)

type Server struct {
	pb.UnimplementedReservationServiceServer
}

func (s *Server) ActivateReservationsGuest(
	c context.Context,
	req *pb.IdRequest,
) (*pb.ActiveReservationsGuestResponse, error) {
	return &pb.ActiveReservationsGuestResponse{
		Reservations: []*pb.Reservation{},
	}, nil
}
