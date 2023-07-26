package services

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/dzoniops/common/pkg/reservation"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dzoniops/reservation-service/db"
	"github.com/dzoniops/reservation-service/utils"
)

func setup() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	req := testcontainers.ContainerRequest{
		Image:        "postgres:12",
		ExposedPorts: []string{"5432/tcp"},
		AutoRemove:   true,
		Env: map[string]string{
			"POSTGRES_USER":     os.Getenv("PGUSER"),
			"POSTGRES_PASSWORD": os.Getenv("PGPASSWORD"),
			"POSTGRES_DB":       os.Getenv("PGDATABASE"),
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	postgres, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		log.Fatal("error:", err)
	}
	dbPort, err := postgres.MappedPort(context.Background(), nat.Port("5432/tcp"))
	if err != nil {
		log.Fatal("error:", err)
	}
	os.Setenv("PGPORT", dbPort.Port())
	db.InitDB()
	utils.InitValidator()
}

func teardown() {
	// db.DB.Delete(&models.Reservation{})
	return
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func TestGetActiveReservationsGuest(t *testing.T) {
	req := reservation.ReserveRequest{
		UserId: 1,
		Reservation: &reservation.Reservation{
			AccommodationId: 1,
			StartDate:       timestamppb.New(time.Now().Add(24 * time.Hour)),
			EndDate:         timestamppb.New(time.Now().Add(48 * time.Hour)),
			NumberOfGuests:  1,
			Status:          reservation.ReservationStatus_RESERVATION_STATUS_PENDING,
			HostId:          2,
		},
	}
	info, err := (&Server{}).Reserve(context.TODO(), &req)

	require.NoError(t, err)
	require.NotEqual(t, info.ReservationId, 0)
}
