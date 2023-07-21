package services

import (
	"context"
	"time"

	pb "github.com/dzoniops/common/pkg/reservation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dzoniops/reservation-service/db"
	_ "github.com/dzoniops/reservation-service/db"
	"github.com/dzoniops/reservation-service/models"
)

type Server struct {
	pb.UnimplementedReservationServiceServer
}

func (s *Server) ActivateReservationsGuest(
	c context.Context,
	req *pb.IdRequest,
) (*pb.ActiveReservationsResponse, error) {
	return &pb.ActiveReservationsResponse{
		Reservations: []*pb.Reservation{{
			AccommodationId: 0,
			StartDate:       &timestamppb.Timestamp{},
			EndDate:         &timestamppb.Timestamp{},
			NumberOfGuests:  0,
			Status:          1,
		}},
	}, nil
}

func (s *Server) ActivateReservationsHost(
	c context.Context,
	req *pb.IdRequest,
) (*pb.ActiveReservationsResponse, error) {
	return &pb.ActiveReservationsResponse{
		Reservations: []*pb.Reservation{},
	}, nil
}

func (s *Server) Reserve(c context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {
	reservation := models.Reservation{
		AccomodationId: req.Reservation.AccommodationId,
		UserId:         req.UserId,
		StartDate:      req.Reservation.StartDate.AsTime(),
		EndDate:        req.Reservation.EndDate.AsTime(),
		Status:         int32(req.Reservation.Status),
	}
	if reservation.EndDate.Before(reservation.StartDate) {
		return nil, status.Error(codes.InvalidArgument, "Start date has to be before End date")
	}
	if s.checkActiveReservations(reservation.StartDate, reservation.EndDate) {
		db.DB.Create(&reservation)
		return &pb.ReserveResponse{
			ReservationId: reservation.ID,
		}, nil
	}
	return nil, status.Error(
		codes.AlreadyExists,
		"Selected dates overlap with existing accepted reservations",
	)
}

func (s *Server) checkActiveReservations(startDate, endDate time.Time) bool {
	var reservations []models.Reservation
	db.DB.Where("start_date > ? AND start_date > ? AND status = 2", startDate, endDate).
		Or("end_date < ? AND end_date < ? AND status = 2", startDate, endDate).Find(&reservations)
	return len(reservations) == 0
}
