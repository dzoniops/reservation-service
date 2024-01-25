package services

import (
	"context"
	"fmt"
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

func (s *Server) FilterAvailableForAccommodations(c context.Context, req *pb.FilterAccommodationsRequest) (*pb.FilterAvailableResponse, error) {
	var result pb.FilterAvailableResponse
	for _, accommodation := range req.Accommodations {
		price, err := s.checkAvailableGetPrice(accommodation.StartDate.AsTime(), accommodation.EndDate.AsTime(), accommodation.AccommodationId)
		if err == nil {
			idPrice := pb.IdPrice{
				Id:    accommodation.AccommodationId,
				Price: price,
			}
			result.IdPrices = append(result.IdPrices, &idPrice)
		}
	}
	return &result, nil
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

	db.DB.Where("status = ? AND host_id = ? AND end_date > ?", models.ACCEPTED, req.Id, time.Now()).
		Find(&reservations)
	return &pb.ActiveReservationsResponse{
		Reservations: mapToPb(reservations),
	}, nil
}

func (s *Server) Reserve(c context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {

	//TODO: uzeti iz accomodation hostId
	//accommodation, err := s.AccommodationClient.GetAccommodation(c, reservation.AccommodationId)
	//if err != nil {
	//	return nil, status.Error(codes.NotFound, "Accommodation not found")
	//}
	reservation := models.Reservation{
		AccommodationId: req.Reservation.AccommodationId,
		UserId:          req.UserId,
		StartDate:       req.Reservation.StartDate.AsTime(),
		EndDate:         req.Reservation.EndDate.AsTime(),
		Status:          models.PENDING,
		NumberOfGuests:  req.Reservation.NumberOfGuests,
		HostId:          req.Reservation.HostId,
	}
	err := utils.Validate.Struct(reservation)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	//if accommodation.MinGuests < reservation.NumberOfGuests ||
	//	accommodation.MaxGuests > reservation.NumberOfGuests {
	//	return nil, status.Error(codes.InvalidArgument, "Invalid number of guests")
	//
	//}
	if s.checkIfAvailable(reservation.StartDate, reservation.EndDate, reservation.AccommodationId) {
		return nil, status.Error(codes.InvalidArgument, "Not available for this date range")
	}
	if s.checkActiveReservations(reservation.StartDate, reservation.EndDate, reservation.AccommodationId) {
		return nil, status.Error(
			codes.AlreadyExists,
			"Selected dates overlap with existing accepted reservations")
	}
	//if accommodation.GetReservationModel() == accommodationpb.ReservationModel_RESERVATION_MODEL_AUTO {
	//	reservation.Status = models.ACCEPTED
	//}

	db.DB.Create(&reservation)
	return &pb.ReserveResponse{
		ReservationId: reservation.ID,
	}, nil
}

func (s *Server) AddAvailable(c context.Context, req *pb.AddAvailableRequest) (*emptypb.Empty, error) {
	//TODO: check if accommodation exists
	//accommodation, err := s.AccommodationClient.GetAccommodation(c, reservation.AccommodationId)
	//if err != nil {
	//	return nil, status.Error(codes.NotFound, "Accommodation not found")
	//}
	startDate := req.StartDate.AsTime()
	endDate := req.EndDate.AsTime()
	if !s.checkAvailableOverlap(startDate, endDate) {
		return nil, status.Error(codes.InvalidArgument, "Overlaps with another available section")
	}
	available := models.Availability{
		AccommodationId: req.AccommodationId,
		Price:           req.Price,
		StartDate:       startDate,
		EndDate:         endDate,
	}
	db.DB.Create(&available)
	return &emptypb.Empty{}, nil
}

func (s *Server) EditAvailable(c context.Context, req *pb.EditAvailableRequest) (*emptypb.Empty, error) {
	var available models.Availability
	startDate := req.StartDate.AsTime()
	endDate := req.EndDate.AsTime()

	if res := db.DB.Where(&models.Availability{ID: req.Id}).First(&available); res.Error != nil {
		return nil, status.Error(codes.NotFound, "Available section not found")
	}
	if !s.checkForExistingAvailableOverlap(startDate, endDate, req.Id, req.AccommodationId) {
		return nil, status.Error(codes.InvalidArgument, "Overlaps with another available section")
	}
	if s.checkActiveReservations(startDate, endDate, req.AccommodationId) {
		return nil, status.Error(codes.InvalidArgument, "Cannot edit selected section while there are active reservation")
	}
	available.StartDate = startDate
	available.EndDate = endDate
	db.DB.Save(&available)

	return &emptypb.Empty{}, nil

}

func (s *Server) checkForExistingAvailableOverlap(startDate, endDate time.Time, id, accommodationId int64) bool {
	available := s.availableExcludingInGivenRange(startDate, endDate, id, accommodationId)
	return len(available) == 0
}

func (s *Server) availableExcludingInGivenRange(startDate, endDate time.Time, id, accommodationId int64) (available []models.Availability) {
	db.DB.Where("start_date < ? and end_date > ? and id != ? and accommodation_id = ?", endDate, startDate, id, accommodationId).
		Find(&available)
	return available
}
func (s *Server) checkAvailableOverlap(startDate, endDate time.Time) bool {
	available := s.availableInGivenRange(startDate, endDate)
	return len(available) == 0
}
func (s *Server) availableInGivenRange(startDate, endDate time.Time) (available []models.Availability) {
	db.DB.Where("start_date < ? and end_date > ?", endDate, startDate).
		Find(&available)
	return available
}
func (s *Server) checkAvailableGetPrice(startDate, endDate time.Time, id int64) (int64, error) {
	var availability models.Availability
	res := db.DB.Where("start_date <= ? AND end_date >= ? AND accommodation_id = ?", startDate, endDate, id).
		First(&availability)
	if res.Error != nil {
		return -1, res.Error
	}
	fmt.Println(availability)
	return availability.Price, nil
}
func (s *Server) checkIfAvailable(startDate, endDate time.Time, accommodationId int64) bool {
	var availability models.Availability
	res := db.DB.Where("start_date <= ? and end_date >= ? and accommodation_id = ?", startDate, endDate, accommodationId).
		First(&availability)
	return res.Error != nil
}
func (s *Server) checkActiveReservations(startDate, endDate time.Time, accommodationId int64) bool {
	reservations := s.reservationsInGivenDateRange(startDate, endDate, models.ACCEPTED, accommodationId)
	return len(reservations) != 0
}

func (s *Server) Accept(c context.Context, req *pb.IdRequest) (*pb.ReserveResponse, error) {
	//TODO: check accommodation
	var reservation models.Reservation
	if res := db.DB.Where(&models.Reservation{ID: req.Id, Status: models.PENDING}).First(&reservation); res.Error != nil {
		return nil, status.Error(codes.NotFound, "Reservation not found")
	}
	db.DB.Model(&reservation).Update("status", models.ACCEPTED)
	return &pb.ReserveResponse{
		ReservationId: reservation.ID,
	}, nil
}

func (s *Server) Decline(c context.Context, req *pb.IdRequest) (*pb.ReserveResponse, error) {
	var reservation models.Reservation
	if res := db.DB.Where(&models.Reservation{ID: req.Id, Status: models.PENDING}).Find(&reservation); res.Error != nil {
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

	dayBefore := reservation.StartDate.Add(-24 * time.Hour)

	if time.Now().After(dayBefore) {
		return nil, status.Error(codes.InvalidArgument, "Cannot cancel reservation day before")
	}
	db.DB.Model(&reservation).Update("status", models.DECLINED)
	return &emptypb.Empty{}, nil
}

func (s *Server) reservationsInGivenDateRange(
	startDate, endDate time.Time,
	status models.ReservationStatus,
	accommodationId int64) (reservations []models.Reservation) {
	db.DB.Where(
		"start_date < ? and end_date > ? and status = ? and accommodation_id = ?", endDate, startDate, status, accommodationId).
		Find(&reservations)
	return reservations
}
func (s *Server) PendingReservationsHost(c context.Context, req *pb.IdRequest) (*pb.PendingReservationsResponse, error) {
	var reservations []models.Reservation
	db.DB.Where(&models.Reservation{Status: models.PENDING, HostId: req.Id}).Find(&reservations)
	return &pb.PendingReservationsResponse{
		Reservations: mapToPb(reservations),
	}, nil

}
func (s *Server) PendingReservationsGuest(c context.Context, req *pb.IdRequest) (*pb.PendingReservationsResponse, error) {
	var reservations []models.Reservation
	db.DB.Where(&models.Reservation{Status: models.PENDING, UserId: req.Id}).Find(&reservations)
	return &pb.PendingReservationsResponse{
		Reservations: mapToPb(reservations),
	}, nil

}
func (s *Server) PendingReservationsAccommodation(c context.Context, req *pb.IdRequest) (*pb.PendingReservationsResponse, error) {
	var reservations []models.Reservation
	db.DB.Where(&models.Reservation{AccommodationId: req.Id, Status: models.PENDING}).Find(&reservations)
	return &pb.PendingReservationsResponse{
		Reservations: mapToPb(reservations),
	}, nil

}

func mapToPb(in []models.Reservation) []*pb.Reservation {
	reservations := make([]*pb.Reservation, len(in))
	for i := range in {
		reservations[i] = &pb.Reservation{
			AccommodationId: in[i].AccommodationId,
			StartDate:       timestamppb.New(in[i].StartDate),
			EndDate:         timestamppb.New(in[i].EndDate),
			NumberOfGuests:  in[i].NumberOfGuests,
			Status:          pb.ReservationStatus(in[i].Status),
		}
	}
	return reservations
}
