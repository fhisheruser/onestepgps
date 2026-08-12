
package demo

import (
	"context"
	"fmt"
	"math"
	"time"

	"fleetview/internal/domain"
)


type vehicle struct {
	id       string
	name     string
	make     string
	model    string
	group    string
	centerLat float64
	centerLng float64
	
	radiusDeg float64

	periodSec float64
	phase     float64

	behaviour int
	odometer  float64
}


type Simulator struct {
	vehicles  []vehicle
	speedUnit string
	clock     domain.Clock
}


func New(speedUnit string, clock domain.Clock) *Simulator {
	if speedUnit == "" {
		speedUnit = "km/h"
	}
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &Simulator{
		speedUnit: speedUnit,
		clock:     clock,
		vehicles: []vehicle{
			{id: "demo-0001", name: "Truck 04 — Harbor Run", make: "Freightliner", model: "M2 106", group: "Logistics", centerLat: 32.7157, centerLng: -117.1611, radiusDeg: 0.030, periodSec: 900, phase: 0.0, behaviour: -1, odometer: 184320},
			{id: "demo-0002", name: "Van 12 — Downtown", make: "Ford", model: "Transit 250", group: "Service", centerLat: 32.7420, centerLng: -117.1300, radiusDeg: 0.018, periodSec: 640, phase: 1.1, behaviour: -1, odometer: 97650},
			{id: "demo-0003", name: "Pickup 07 — North County", make: "Chevrolet", model: "Silverado 2500", group: "Field Ops", centerLat: 33.1581, centerLng: -117.3506, radiusDeg: 0.042, periodSec: 1100, phase: 2.3, behaviour: -1, odometer: 233110},
			{id: "demo-0004", name: "Bus 02 — Campus Shuttle", make: "Blue Bird", model: "Vision", group: "Transit", centerLat: 32.8801, centerLng: -117.2340, radiusDeg: 0.012, periodSec: 480, phase: 0.7, behaviour: -1, odometer: 412008},
			{id: "demo-0005", name: "Van 19 — Airport Loop", make: "Mercedes-Benz", model: "Sprinter", group: "Service", centerLat: 32.7338, centerLng: -117.1933, radiusDeg: 0.015, periodSec: 520, phase: 3.4, behaviour: -1, odometer: 61240},
			{id: "demo-0006", name: "Truck 11 — Otay Mesa", make: "Peterbilt", model: "579", group: "Logistics", centerLat: 32.5540, centerLng: -116.9700, radiusDeg: 0.026, periodSec: 980, phase: 4.8, behaviour: -1, odometer: 508720},
			{id: "demo-0007", name: "Car 21 — Sales West", make: "Toyota", model: "Camry", group: "Sales", centerLat: 32.8328, centerLng: -117.2713, radiusDeg: 0.020, periodSec: 700, phase: 5.6, behaviour: -1, odometer: 44190},
			{id: "demo-0008", name: "Truck 08 — Depot (parked)", make: "Isuzu", model: "NPR-HD", group: "Logistics", centerLat: 32.7960, centerLng: -117.0810, radiusDeg: 0.0, periodSec: 1, phase: 0, behaviour: 2, odometer: 129480},
			{id: "demo-0009", name: "Van 05 — Yard (idling)", make: "RAM", model: "ProMaster", group: "Service", centerLat: 32.6890, centerLng: -117.1090, radiusDeg: 0.0006, periodSec: 240, phase: 2.0, behaviour: 1, odometer: 88010},
			{id: "demo-0010", name: "Pickup 14 — Escondido", make: "GMC", model: "Sierra 1500", group: "Field Ops", centerLat: 33.1192, centerLng: -117.0864, radiusDeg: 0.034, periodSec: 1250, phase: 1.9, behaviour: -1, odometer: 156300},
		},
	}
}


func (s *Simulator) Name() string { return "demo-simulator" }


func (s *Simulator) FetchDevices(_ context.Context) ([]domain.Device, error) {
	now := s.clock.Now()
	devices := make([]domain.Device, 0, len(s.vehicles))
	for i := range s.vehicles {
		devices = append(devices, s.vehicles[i].at(now, s.speedUnit))
	}
	return devices, nil
}


func (v vehicle) at(t time.Time, speedUnit string) domain.Device {
	seconds := float64(t.UnixNano()) / float64(time.Second)
	omega := 2 * math.Pi / v.periodSec

	lat, lng := v.positionAt(seconds, omega)
	prevLat, prevLng := v.positionAt(seconds-1, omega)


	speedKPH := haversineMeters(prevLat, prevLng, lat, lng) * 3.6
	heading := bearing(prevLat, prevLng, lat, lng)

	status := domain.DriveStatusDriving
	switch v.behaviour {
	case 1:
		status = domain.DriveStatusIdle
		speedKPH = 0
	case 2:
		status = domain.DriveStatusOff
		speedKPH = 0
		heading = 0
	default:
	
		cycle := math.Mod(seconds/v.periodSec+v.phase, 1.0)
		if cycle > 0.82 {
			status = domain.DriveStatusIdle
			speedKPH = 0
		}
	}

	online := v.behaviour != 2 || math.Mod(seconds/600, 2) < 1.5

	return domain.Device{
		ID:        v.id,
		Name:      v.name,
		Make:      v.make,
		Model:     v.model,
		FactoryID: fmt.Sprintf("SIM-%s", v.id[len(v.id)-4:]),
		Online:    online,
		Active:    true,
		Position: domain.Position{
			Lat:        lat,
			Lng:        lng,
			Altitude:   40 + 30*math.Sin(seconds/500+v.phase),
			Heading:    heading,
			Speed:      convertSpeed(speedKPH, speedUnit),
			SpeedUnit:  speedUnit,
			RecordedAt: t,
		},
		DriveStatus:         status,
		DriveStatusDuration: time.Duration(math.Mod(seconds, 3600)) * time.Second,
		DriveStatusSince:    t.Add(-time.Duration(math.Mod(seconds, 3600)) * time.Second),
		Odometer:            v.odometer + math.Mod(seconds, 86400)/120,
		OdometerUnit:        "km",
		Groups:              []string{v.group},
	}
}

func (v vehicle) positionAt(seconds, omega float64) (lat, lng float64) {
	angle := seconds*omega + v.phase
	lat = v.centerLat + v.radiusDeg*math.Sin(angle)
	
	lngScale := 1 / math.Max(0.1, math.Cos(v.centerLat*math.Pi/180))
	lng = v.centerLng + v.radiusDeg*math.Cos(angle)*lngScale
	return lat, lng
}


func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Asin(math.Min(1, math.Sqrt(a)))
}


func bearing(lat1, lng1, lat2, lng2 float64) float64 {
	rad := math.Pi / 180
	y := math.Sin((lng2-lng1)*rad) * math.Cos(lat2*rad)
	x := math.Cos(lat1*rad)*math.Sin(lat2*rad) -
		math.Sin(lat1*rad)*math.Cos(lat2*rad)*math.Cos((lng2-lng1)*rad)
	deg := math.Atan2(y, x) / rad
	return math.Mod(deg+360, 360)
}


func convertSpeed(kph float64, unit string) float64 {
	switch unit {
	case "mph":
		return kph / 1.609344
	case "kn":
		return kph / 1.852
	default:
		return kph
	}
}
