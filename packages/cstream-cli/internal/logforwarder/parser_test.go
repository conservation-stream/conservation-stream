package logforwarder

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/observiq/go-syslog/v3/rfc5424"
)

func severityFromPRI(pri uint8) *uint8 {
	sev := pri % 8
	return &sev
}

func facilityFromPRI(pri uint8) *uint8 {
	fac := pri / 8
	return &fac
}

func TestParseSyslogLine_OK(t *testing.T) {
	parser := rfc5424.NewParser(rfc5424.WithBestEffort())
	now := time.Date(2025, 3, 11, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		line string
		want LogRecord
	}{
		{
			name: "valid RFC5424",
			line: `<165>1 2025-03-11T11:59:00Z hostname app - - - message`,
			want: LogRecord{
				Hostname:   "hostname",
				Source:     "app",
				Message:    "message",
				TS:         "2025-03-11T11:59:00Z",
				Severity:   severityFromPRI(165),
				Facility:   facilityFromPRI(165),
				ReceivedAt: now.Format(time.RFC3339Nano),
				Raw:        `<165>1 2025-03-11T11:59:00Z hostname app - - - message`,
			},
		},
		{
			name: "unicode and escaped characters in message",
			line: `<190>1 2025-03-11T11:59:01Z host app - - - User=zoe action="save" path="C:\\Program Files\\App" note="emoji 🚀 and euro €" status=ok	end`,
			want: LogRecord{
				Hostname:   "host",
				Source:     "app",
				Message:    `User=zoe action="save" path="C:\\Program Files\\App" note="emoji 🚀 and euro €" status=ok	end`,
				TS:         "2025-03-11T11:59:01Z",
				Severity:   severityFromPRI(190),
				Facility:   facilityFromPRI(190),
				ReceivedAt: now.Format(time.RFC3339Nano),
				Raw:        `<190>1 2025-03-11T11:59:01Z host app - - - User=zoe action="save" path="C:\\Program Files\\App" note="emoji 🚀 and euro €" status=ok	end`,
			},
		},
		{
			name: "structured data is flattened",
			line: `<34>1 2025-03-11T11:59:02Z web api 4242 evt42 [exampleSDID@32473 iut="3" eventSource="Application"] request completed`,
			want: LogRecord{
				Hostname:   "web",
				Source:     "api",
				ProcID:     "4242",
				MsgID:      "evt42",
				Message:    "request completed",
				TS:         "2025-03-11T11:59:02Z",
				Severity:   severityFromPRI(34),
				Facility:   facilityFromPRI(34),
				Structured: map[string]string{"exampleSDID@32473.iut": "3", "exampleSDID@32473.eventSource": "Application"},
				ReceivedAt: now.Format(time.RFC3339Nano),
				Raw:        `<34>1 2025-03-11T11:59:02Z web api 4242 evt42 [exampleSDID@32473 iut="3" eventSource="Application"] request completed`,
			},
		},
		{
			name: "missing timestamp falls back to received time",
			line: `<13>1 - host app - - - no timestamp`,
			want: LogRecord{
				Hostname:   "host",
				Source:     "app",
				Message:    "no timestamp",
				TS:         now.Format(time.RFC3339Nano),
				Severity:   severityFromPRI(13),
				Facility:   facilityFromPRI(13),
				ReceivedAt: now.Format(time.RFC3339Nano),
				Raw:        `<13>1 - host app - - - no timestamp`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSyslogLine(parser, []byte(tt.line), now)
			if err != nil {
				t.Fatalf("ParseSyslogLine(%q) unexpected error: %v", tt.line, err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ParseSyslogLine(%q) mismatch (-want +got):\n%s", tt.line, diff)
			}
		})
	}
}

func TestParseSyslogLine_Error(t *testing.T) {
	parser := rfc5424.NewParser(rfc5424.WithBestEffort())
	now := time.Date(2025, 3, 11, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		line string
	}{
		{name: "invalid line", line: `not valid syslog`},
		{name: "empty", line: ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSyslogLine(parser, []byte(tt.line), now)
			if err == nil {
				t.Errorf("ParseSyslogLine(%q) error = nil, want non-nil", tt.line)
			}
		})
	}
}
