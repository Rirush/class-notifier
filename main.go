package main

import (
	"encoding/json"
	"fmt"
	"github.com/Rirush/forlabs"
	"gopkg.in/tucnak/telebot.v2"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	settings := telebot.Settings{
		Token: os.Getenv("TELEGRAM_TOKEN"),
	}

	if os.Getenv("USE_POLLING") == "yes" {
		settings.Poller = &telebot.LongPoller{Timeout: 10 * time.Second}
	}

	b, err := telebot.NewBot(settings)
	if err != nil {
		log.Fatal("cannot connect to telegram:", err)
	}

	f, err := forlabs.Authenticate(os.Getenv("FORLABS_USERNAME"), os.Getenv("FORLABS_PASSWORD"))
	if err != nil {
		log.Fatal("cannot authenticate into forlabs:", err)
	}

	if os.Getenv("USE_POLLING") != "yes" {
		http.HandleFunc("/"+os.Getenv("TELEGRAM_WEBHOOK_KEY"), func(writer http.ResponseWriter, request *http.Request) {
			upd := telebot.Update{}
			data, err := io.ReadAll(request.Body)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			if err := json.Unmarshal(data, &upd); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			b.ProcessUpdate(upd)
			writer.WriteHeader(http.StatusOK)
		})
		go http.ListenAndServe(":"+os.Getenv("PORT"), nil)
		err = b.SetWebhook(&telebot.Webhook{
			Endpoint: &telebot.WebhookEndpoint{
				PublicURL: os.Getenv("TELEGRAM_WEBHOOK_HOST")+"/"+os.Getenv("TELEGRAM_WEBHOOK_KEY"),
			},
		})
		if err != nil {
			log.Fatalln("cannot set webhook:", err)
		}
	}

	grid, err := f.GetGrid()
	if err != nil {
		log.Println("cannot query forlabs for schedule grid:", err)
		return
	}
	schedule, err := f.GetSchedule(0)
	if err != nil {
		log.Println("cannot query forlabs for schedule:", err)
		return
	}

	b.Handle("/schedule", func(m *telebot.Message) {
		now := time.Now()
		_, week := now.ISOWeek()
		upper := week % 2 == 0
		weekday := int(now.Weekday())
		if upper {
			weekday += 7
		}
		result := "📅 Расписание на сегодня:\n\n"
		lastPos := 0
		seq := 1
		for _, e := range schedule.Entries {
			if e.Day == weekday {
				if e.Position == lastPos {
					continue
				}
				lastPos = e.Position
				result += fmt.Sprintf("%d) %s в %s (%s - %s)\n", seq, e.StudyName, e.RoomName, grid.Positions[e.Position-1].Start, grid.Positions[e.Position-1].End)
				seq++
			}
		}
		_, err = b.Reply(m, result)
		if err != nil {
			log.Println("cannot send message:", err)
			return
		}
	})

	go b.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	<-sigs
	signal.Stop(sigs)
	b.Stop()
}
