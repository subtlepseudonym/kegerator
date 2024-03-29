package kegerator

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/subtlepseudonym/kegerator/prometheus"

	"github.com/d2r2/go-dht"
)

const (
	defaultDHTAttachRetries = 4
	defaultDHTReadRetries   = 10
	defaultDHTReadInterval  = 10 * time.Second
	defaultTemperatureLimit = 100.0 // ignore temperature values over 100C
)

// These values are used for writing to and from file
var (
	dhtModels map[string]dht.SensorType = map[string]dht.SensorType{
		"dht22": dht.DHT22,
	}
	dhtIndex map[dht.SensorType]string = map[dht.SensorType]string{
		dht.DHT22: "dht22",
	}
)

func GetDHTModel(name string) (dht.SensorType, error) {
	model, ok := dhtModels[name]
	if ok {
		return model, nil
	}
	return dht.SensorType(0), fmt.Errorf("unknown model: %s", name)
}

type DHT struct {
	model  dht.SensorType
	pin    int
	ticker *time.Ticker
	mu     sync.Mutex
	stop   chan struct{}

	Temperature float32
	Humidity    float32
	Retries     int
}

func NewDHT(sensor dht.SensorType, interval time.Duration) *DHT {
	return &DHT{
		model:  sensor,
		ticker: time.NewTicker(interval),
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
	d.Humidity = humidity
	d.Retries = retries
	prometheus.DHTHumidity.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Set(float64(humidity / 100.0))
	prometheus.DHTRetries.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Add(float64(retries))

	if temperature < defaultTemperatureLimit {
		d.Temperature = temperature
		prometheus.DHTTemperature.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Set(float64(temperature))
	}

	return nil
}

func (d *DHT) Detach() error {
	d.ticker.Stop()
	return nil
}

func (d *DHT) Start(update func(context.Context)) {
	if d.stop != nil {
		return
	}

	d.stop = make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-d.ticker.C:
				update(ctx)
			case <-d.stop:
				cancel()
				return
			}
		}
	}()
}

func (d *DHT) Update(ctx context.Context) {
	temp, humid, retries, err := dht.ReadDHTxxWithContextAndRetry(
		ctx,
		d.model,
		d.pin,
		false,
		defaultDHTReadRetries,
	)
	if err != nil {
		log.Println("ERR:", err)
		return
	}

	if temp > defaultTemperatureLimit {
		log.Printf(
			"WARN: pin %d: recorded temperature exceeds limit: %.2f > %.2f\n",
			d.pin,
			temp,
			defaultTemperatureLimit,
		)
		return
	}

	d.mu.Lock()
	d.Temperature = temp
	d.Humidity = humid
	d.Retries = retries

	prometheus.DHTTemperature.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Set(float64(temp))
	prometheus.DHTHumidity.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Set(float64(humid / 100.0))
	prometheus.DHTRetries.WithLabelValues(strconv.Itoa(d.pin), d.Model()).Add(float64(retries))
	d.mu.Unlock()
}

func (d *DHT) Stop() {
	if d.stop == nil {
		return
	}
	close(d.stop)
	d.ticker.Stop()
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
