package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/d2r2/go-dht"
)

const (
	defaultDHTAttachRetries = 4
	defaultDHTReadRetries   = 10
	defaultDHTReadInterval  = 10 * time.Second
)

var (
	dhtIndex map[dht.SensorType]string = map[dht.SensorType]string{
		dht.DHT22: "dht22",
	}
)

type DHT struct {
	model dht.SensorType
	pin   int
	timer *time.Timer
	mu    sync.Mutex
	stop  chan struct{}

	Temperature float32
	Humidity    float32
	Retries     int
}

func NewDHT(sensor dht.SensorType, interval time.Duration) *DHT {
	return &DHT{
		model: sensor,
		timer: time.NewTimer(interval),
	}
}

func (d *DHT) Attach(pin int) error {
	temperature, humidity, retries, err := dht.ReadDHTxxWithRetry(
		d.model,
		pin,
		false,
		defaultDHTAttachRetries,
	)
	if err != nil {
		return fmt.Errorf("open dht: %w", err)
	}

	d.pin = pin
	d.Temperature = temperature
	d.Humidity = humidity
	d.Retries = retries
	DHTTemperature.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Set(float64(temperature))
	DHTHumidity.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Set(float64(humidity / 100.0))
	DHTRetries.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Add(float64(retries))
	return nil
}

func (d *DHT) Detach() error {
	d.timer.Stop()
	return nil
}

func (d *DHT) Start() {
	if d.stop != nil {
		return
	}

	d.stop = make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-d.timer.C:
				temp, humid, retries, err := dht.ReadDHTxxWithContextAndRetry(
					ctx,
					d.model,
					d.pin,
					false,
					defaultDHTReadRetries,
				)
				if err != nil {
					fmt.Println("ERR:", err)
					continue
				}

				d.mu.Lock()
				d.Temperature = temp
				d.Humidity = humid
				d.Retries = retries

				DHTTemperature.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Set(float64(temp))
				DHTHumidity.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Set(float64(humid / 100.0))
				DHTRetries.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Add(float64(retries))
				d.mu.Unlock()
			case <-d.stop:
				cancel()
				return
			}
		}
	}()
}

func (d *DHT) Stop() {
	if d.stop == nil {
		return
	}
	close(d.stop)
	d.timer.Stop()
}

func (d *DHT) Lock() {
	d.mu.Lock()
}

func (d *DHT) Unlock() {
	d.mu.Unlock()
}

func (d *DHT) Model() string {
	return dhtIndex[d.model]
}

func (d *DHT) Pin() int {
	return d.pin
}