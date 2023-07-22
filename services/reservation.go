package services

import (
	"context"
	"log"
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
	var reservations []models.Reservation

	db.DB.Where(&models.Reservation{Status: models.ACTIVE, UserId: req.Id}).
		Find(&reservations)
	return &pb.ActiveReservationsResponse{
		Reservations: mapToPb(reservations),
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
		Status:         models.ReservationStatus(req.Reservation.Status),
		NumberOfGuests: req.Reservation.NumberOfGuests,
	}
	// if reservation.StartDate.Before(time.Now()) || reservation.EndDate.Before(time.Now()) {
	// 	return nil, status.Error(codes.InvalidArgument, "Start or End date have to be in future")
	// }
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
	db.DB.Where("start_date < ? and end_date > ? and status = ?", endDate, startDate, models.ACCEPTED).
		Find(&reservations)
	// db.DB.Where("start_date > ? AND start_date > ? AND status = 2", startDate, endDate).
	// 	Or("end_date < ? AND end_date < ? AND status = 2", startDate, endDate).Find(&reservations)
	log.Printf("%v", reservations)
	return len(reservations) == 0
}

func mapToPb(in []models.Reservation) []*pb.Reservation {
	vals := make([]*pb.Reservation, len(in))
	for i := range in {
		vals[i] = &pb.Reservation{
			AccommodationId: in[i].AccomodationId,
			StartDate:       timestamppb.New(in[i].StartDate),
			EndDate:         timestamppb.New(in[i].EndDate),
			NumberOfGuests:  in[i].NumberOfGuests,
			Status:          pb.ReservationStatus(in[i].Status),
		}
	}
	return vals
}
