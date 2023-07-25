package batchnotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"strings"

	"github.com/mhale/smtpd"
)

func SystemName(to string) string {
	account := strings.Split(to, "@")[0]
	return account
}

type SlackMessage struct {
	Text string `json:"text"`
}

type Event struct {
	System       string
	JobName      string
	Status       string
	ExitCode     int
	Unrecognized string
}

func (e *Event) String() string {
	if e.Unrecognized != "" {
		return fmt.Sprintf("[%s] %s", e.System, e.Unrecognized)
	}
	return fmt.Sprintf("[%s] %s - %s", e.System, e.JobName, strings.ToLower(e.Status))
}

func parseEventPBS(m *mail.Message) (*Event, error) {
	event := &Event{}

	body, err := io.ReadAll(m.Body)
	if err != nil {
		return nil, fmt.Errorf("read message body: %w", err)
	}
	sp := strings.Split(string(body), "\n")
	event.JobName = strings.TrimRight(strings.TrimPrefix(sp[1], "Job Name:   "), "\n\r")
	event.Status = sp[2]

	return event, nil
}

func parseEventSlurm(m *mail.Message) (*Event, error) {
	event := &Event{}

	subject := m.Header.Get("Subject")
	slurmIndex := strings.Index(strings.ToLower(subject), "slurm")
	if slurmIndex == -1 {
		return nil, fmt.Errorf("no slurm indicator in the subject")
	}
	subject = subject[slurmIndex:]

	sp := strings.Split(subject, " ")
	event.JobName = strings.Split(sp[2], "=")[1]
	event.Status = strings.TrimRight(sp[3], ",")

	return event, nil
}

func ParseEvent(data []byte) (*Event, error) {
	r := bytes.NewReader(data)
	m, err := mail.ReadMessage(r)
	if err != nil {
		return nil, fmt.Errorf("read message: %w", err)
	}

	header := m.Header
	subject := header.Get("Subject")

	var event *Event
	switch {
	case strings.Contains(strings.ToLower(subject), "pbs"):
		event, err = parseEventPBS(m)
		if err != nil {
			return nil, fmt.Errorf("parse message: %w", err)
		}
	case strings.Contains(strings.ToLower(subject), "slurm"):
		event, err = parseEventSlurm(m)
		if err != nil {
			return nil, fmt.Errorf("parse message: %w", err)
		}
	default:
		body, err := io.ReadAll(m.Body)
		if err != nil {
			return nil, fmt.Errorf("read message body: %w", err)
		}
		event = &Event{
			Unrecognized: fmt.Sprintf("Subject: %s\nBody:\n%s", subject, body),
		}
	}

	event.System = SystemName(header.Get("To"))

	return event, nil
}

func (config *Config) mailHandler(origin net.Addr, from string, to []string, data []byte) error {
	log.Printf("Received mail from %s for %s", from, strings.Join(to, ";"))

	event, err := ParseEvent(data)
	if err != nil {
		return fmt.Errorf("parse event: %w", err)
	}

	slackMessage := SlackMessage{
		Text: event.String(),
	}
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(slackMessage); err != nil {
		return fmt.Errorf("encode JSON error: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, config.SlackURL, buf)
	if err != nil {
		return fmt.Errorf("NewRequest error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("response error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status error: %s", resp.Status)
	}

	return nil
}

func (config *Config) rcptHandler(remoteAddr net.Addr, from string, to string) bool {
	log.Printf("mail from %s", from)
	for _, af := range config.AllowedFrom {
		if af == from {
			return true
		}
	}
	return false
}

type Config struct {
	AllowedFrom  []string
	SlackURL     string
	MailHostname string
}

func Run(config *Config) error {
	srv := &smtpd.Server{
		Addr:        ":25",
		Handler:     config.mailHandler,
		HandlerRcpt: config.rcptHandler,
		Appname:     "batch-notify",
		Hostname:    config.MailHostname,
		TLSRequired: false,
	}

	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("ListenAndServe: %w", err)
	}

	return nil
}
