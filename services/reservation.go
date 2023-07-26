package services

import (
	"context"
	"time"

	pb "github.com/dzoniops/common/pkg/reservation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dzoniops/reservation-service/client"
	_ "github.com/dzoniops/reservation-service/client"
	"github.com/dzoniops/reservation-service/db"
	"github.com/dzoniops/reservation-service/models"
	"github.com/dzoniops/reservation-service/utils"
)

type Server struct {
	pb.UnimplementedReservationServiceServer
	AccommodationClient client.AccommodationClient
}

func (s *Server) ActiveReservationsGuest(
	c context.Context,
	req *pb.IdRequest,
) (*pb.ActiveReservationsResponse, error) {
	var reservations []models.Reservation

	db.DB.Where(&models.Reservation{Status: models.ACCEPTED, UserId: req.Id}).Find(&reservations)
	return &pb.ActiveReservationsResponse{
		Reservations: mapToPb(reservations),
	}, nil
}

func (s *Server) ActiveReservationsHost(
	c context.Context,
	req *pb.IdRequest,
) (*pb.ActiveReservationsResponse, error) {
	var reservations []models.Reservation

	db.DB.Where("status = ? AND host_id = ? AND start_date > ?", models.ACCEPTED, req.Id, time.Now()).
		Find(&reservations)
	return &pb.ActiveReservationsResponse{
		Reservations: mapToPb(reservations),
	}, nil
}

func (s *Server) Reserve(c context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {
	reservation := models.Reservation{
		AccomodationId: req.Reservation.AccommodationId,
		UserId:         req.UserId,
		StartDate:      req.Reservation.StartDate.AsTime(),
		EndDate:        req.Reservation.EndDate.AsTime(),
		Status:         models.PENDING,
		NumberOfGuests: req.Reservation.NumberOfGuests,
		HostId:         req.Reservation.HostId,
	}
	err := utils.Validate.Struct(reservation)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// accommodation, err := s.AccommodationClient.GetAccommodation(c, reservation.AccomodationId)
	// if err != nil {
	// 	return nil, status.Error(codes.NotFound, "Accommodation not found")
	// }
	// if accommodation.MinGuests < reservation.NumberOfGuests ||
	// 	accommodation.MaxGuests > reservation.NumberOfGuests {
	// 	return nil, status.Error(codes.InvalidArgument, "Invalid number of guests")
	// }
	// if accommodation.GetReservationModel() == accommodation_pb.ReservationModel_RESERVATION_MODEL_AUTO {
	// 	reservation.Status = models.ACCEPTED
	// }

	if s.checkActiveReservations(reservation.StartDate, reservation.EndDate) {
		return nil, status.Error(
			codes.AlreadyExists,
			"Selected dates overlap with existing accepted reservations")
	}
	db.DB.Create(&reservation)
	return &pb.ReserveResponse{
		ReservationId: reservation.ID,
	}, nil
}

func (s *Server) checkActiveReservations(startDate, endDate time.Time) bool {
	reservations := s.reservationsInGivenDateRange(startDate, endDate, models.ACCEPTED)
	return len(reservations) != 0
}

func (s *Server) Accept(c context.Context, req *pb.IdRequest) (*pb.ReserveResponse, error) {
	var reservation models.Reservation
	if res := db.DB.Where(&models.Reservation{ID: req.Id}).First(&reservation); res.Error != nil {
		return nil, status.Error(codes.NotFound, "Reservation not found")
	}
	db.DB.Model(&reservation).Update("status", models.ACCEPTED)
	// change all other overlapping to declined
	reservations := s.reservationsInGivenDateRange(
		reservation.StartDate,
		reservation.EndDate,
		models.PENDING,
	)
	for _, r := range reservations {
		db.DB.Model(&r).Update("status", models.DECLINED)
	}
	return &pb.ReserveResponse{
		ReservationId: reservation.ID,
	}, nil
}

func (s *Server) Decline(c context.Context, req *pb.IdRequest) (*pb.ReserveResponse, error) {
	var reservation models.Reservation
	if res := db.DB.Where(&models.Reservation{ID: req.Id}).First(&reservation); res.Error != nil {
		return nil, status.Error(codes.NotFound, "Reservation not found")
	}
	db.DB.Model(&reservation).Update("status", models.DECLINED)
	return &pb.ReserveResponse{
		ReservationId: reservation.ID,
	}, nil
}

func (s *Server) DeleteReservation(c context.Context, req *pb.IdRequest) (*emptypb.Empty, error) {
	var reservation models.Reservation
	if res := db.DB.Where(&models.Reservation{ID: req.Id}).First(&reservation); res.Error != nil {
		return nil, status.Error(codes.NotFound, "Reservation not found")
	}

	tomorrow := reservation.StartDate.Add(24 * time.Hour)

	if reservation.StartDate.Equal(time.Now()) || tomorrow.Equal(time.Now()) {
		return nil, status.Error(codes.InvalidArgument, "Cannot cancel reservation day before")
	}
	db.DB.Model(&reservation).Update("status", models.DECLINED)
	return &emptypb.Empty{}, nil
}

func (s *Server) reservationsInGivenDateRange(
	startDate, endDate time.Time,
	status models.ReservationStatus,
) (reservations []models.Reservation) {
	db.DB.Where("start_date < ? and end_date > ? and status = ?", endDate, startDate, status).
		Find(&reservations)
	return reservations
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
