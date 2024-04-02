package modes

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"

	. "logdy/models"
)

func GenerateRandomData(jsonFormat bool, numPerSec int, ch chan Message, ctx context.Context) {

	if numPerSec > 100 {
		numPerSec = 100
	}

	if numPerSec <= 0 {
		return // produce no data, so just leave
	}

	for {

		if ctx.Err() != nil {
			return
		}

		var msg string
		if jsonFormat {
			msg = generateJsonRandomData()
		} else {
			msg = generateTextRandomData()
		}

		mo := MessageOrigin{}

		if rand.Intn(100) >= 50 {
			mo.File = []string{"foo1.log", "foo2.log", "foo3.log"}[rand.Intn(3)]
		} else {
			mo.Port = []string{"4356", "4333", "4262"}[rand.Intn(3)]
		}
		if rand.Intn(100) >= 90 {
			mo.File = ""
			mo.Port = ""
		}

		produce(ch, msg, MessageTypeStdout, &mo)
		time.Sleep(time.Duration((1 / float64(numPerSec)) * float64(time.Second)))
	}

}

func generateTextRandomData() string {
	return strings.Join([]string{
		time.Now().Format("15:04:05.0000"),
		gofakeit.UUID(),
		gofakeit.DomainName(),
		gofakeit.IPv4Address(),
		gofakeit.URL(),
		gofakeit.LogLevel("log"),
		gofakeit.UserAgent(),
		gofakeit.HTTPMethod(),
	}, " | ")
}

func generateJsonRandomData() string {
	val, _ := json.Marshal(map[string]string{
		"ts":     time.Now().Format("15:04:05.0000"),
		"uuid":   gofakeit.UUID(),
		"domain": gofakeit.DomainName(),
		"ipv4":   gofakeit.IPv4Address(),
		"url":    gofakeit.URL(),
		"level":  gofakeit.LogLevel("log"),
		"ua":     gofakeit.UserAgent(),
		"method": gofakeit.HTTPMethod(),
	})

	return string(val)
}
