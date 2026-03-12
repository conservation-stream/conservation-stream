package logforwarder

import (
	"fmt"
	"time"

	syslog "github.com/observiq/go-syslog/v3"
	"github.com/observiq/go-syslog/v3/rfc5424"
)

// ParseSyslogLine parses one RFC5424 syslog line into a LogRecord using the given parser.
// now is used for ReceivedAt (and for TS when the message has no timestamp); pass time.Now().UTC() in production.
func ParseSyslogLine(parser syslog.Machine, line []byte, now time.Time) (LogRecord, error) {
	msg, err := parser.Parse(line)
	if err != nil {
		return LogRecord{}, fmt.Errorf("parse syslog: %w", err)
	}
	if msg == nil {
		return LogRecord{}, fmt.Errorf("parse syslog: nil message")
	}
	syslogMsg, ok := msg.(*rfc5424.SyslogMessage)
	if !ok {
		return LogRecord{}, fmt.Errorf("parse syslog: expected *rfc5424.SyslogMessage, got %T", msg)
	}
	rec := LogRecord{
		TS:         timestampString(syslogMsg.Timestamp),
		Hostname:   deref(syslogMsg.Hostname),
		Source:     deref(syslogMsg.Appname),
		Message:    deref(syslogMsg.Message),
		Severity:   syslogMsg.Severity,
		Facility:   syslogMsg.Facility,
		ProcID:     deref(syslogMsg.ProcID),
		MsgID:      deref(syslogMsg.MsgID),
		Structured: flattenStructured(syslogMsg.StructuredData),
		Raw:        string(line),
		ReceivedAt: now.Format(time.RFC3339Nano),
	}
	if rec.TS == "" {
		rec.TS = rec.ReceivedAt
	}
	return rec, nil
}
