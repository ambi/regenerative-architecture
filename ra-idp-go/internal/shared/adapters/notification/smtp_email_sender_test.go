package notification

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
)

func TestSMTPEmailSenderSendsPlaintextMessage(t *testing.T) {
	t.Parallel()
	server := newFakeSMTPServer(t)
	defer server.Close()

	sender := NewSMTPEmailSender(SMTPEmailSenderConfig{
		Host:    server.host,
		Port:    server.port,
		From:    "noreply@ra-idp.test",
		TLSMode: SMTPTLSNone,
		Hello:   "test.local",
		Timeout: time.Second,
	})
	ok := sender.SendEmail(context.Background(), authnports.EmailMessage{
		To:      "alice@example.com",
		Subject: "パスワードリセット",
		Text:    "リセットリンク: https://example.com/reset?token=abc",
	})
	if !ok {
		t.Fatalf("SendEmail returned false; transcript=%v", server.transcript())
	}

	transcript := server.transcript()
	requireLineContains(t, transcript, "MAIL FROM:<noreply@ra-idp.test>")
	requireLineContains(t, transcript, "RCPT TO:<alice@example.com>")

	body := server.dataBody()
	requireBodyContains(t, body, "From: <noreply@ra-idp.test>")
	requireBodyContains(t, body, "To: <alice@example.com>")
	requireBodyContains(t, body, "MIME-Version: 1.0")
	requireBodyContains(t, body, "Content-Type: text/plain; charset=utf-8")
	requireBodyContains(t, body, "Content-Transfer-Encoding: base64")
	requireBodyContains(t, body, encodeMIMEBody("リセットリンク: https://example.com/reset?token=abc"))
	requireBodyNotContains(t, body, "リセットリンク")
	requireBodyContains(t, body, "Subject: =?utf-8?")
}

func TestSMTPEmailSenderRejectsPLAINAuthOverPlaintext(t *testing.T) {
	t.Parallel()
	server := newFakeSMTPServer(t)
	defer server.Close()

	sender := NewSMTPEmailSender(SMTPEmailSenderConfig{
		Host: server.host, Port: server.port,
		Username: "u", Password: "p",
		From: "noreply@ra-idp.test", TLSMode: SMTPTLSNone, Timeout: time.Second,
	})
	ok := sender.SendEmail(context.Background(), authnports.EmailMessage{
		To: "alice@example.com", Subject: "x", Text: "y",
	})
	if ok {
		t.Fatal("SendEmail should fail when PLAIN auth attempted over cleartext")
	}
	for _, line := range server.transcript() {
		if strings.HasPrefix(line, "AUTH PLAIN") {
			t.Fatalf("client sent AUTH PLAIN over cleartext: %q", line)
		}
	}
}

func TestSMTPEmailSenderReturnsFalseOnDialFailure(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close()
	sender := NewSMTPEmailSender(SMTPEmailSenderConfig{
		Host: "127.0.0.1", Port: addr.Port,
		From: "noreply@ra-idp.test", TLSMode: SMTPTLSNone, Timeout: 200 * time.Millisecond,
	})
	if sender.SendEmail(context.Background(), authnports.EmailMessage{To: "x@y", Subject: "s", Text: "t"}) {
		t.Fatal("SendEmail should return false when dial fails")
	}
}

func TestBuildRFC5322MessageMultipart(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	body, err := buildRFC5322Message("noreply@ra-idp.test", authnports.EmailMessage{
		To: "bob@example.com", Subject: "verify", Text: "plain body", HTML: "<p>html body</p>",
	}, now)
	if err != nil {
		t.Fatalf("buildRFC5322Message: %v", err)
	}
	for _, want := range []string{
		"From: <noreply@ra-idp.test>",
		"To: <bob@example.com>",
		"Subject: verify",
		"Date: Mon, 15 Jun 2026 12:00:00 +0000",
		"@ra-idp.test>",
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Type: text/html; charset=utf-8",
		"Content-Transfer-Encoding: base64",
		encodeMIMEBody("plain body"),
		encodeMIMEBody("<p>html body</p>"),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in body:\n%s", want, body)
		}
	}
	for _, unsafe := range []string{"plain body", "<p>html body</p>"} {
		if strings.Contains(body, unsafe) {
			t.Errorf("raw content %q reached SMTP DATA:\n%s", unsafe, body)
		}
	}
}

func TestBuildRFC5322MessageHTMLOnly(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	body, err := buildRFC5322Message("noreply@ra-idp.test", authnports.EmailMessage{
		To: "bob@example.com", Subject: "x", HTML: "<p>only html</p>",
	}, now)
	if err != nil {
		t.Fatalf("buildRFC5322Message: %v", err)
	}
	if !strings.Contains(body, "Content-Type: text/html; charset=utf-8") {
		t.Errorf("expected single-part text/html, got:\n%s", body)
	}
	if !strings.Contains(body, "Content-Transfer-Encoding: base64") {
		t.Errorf("expected base64 transfer encoding, got:\n%s", body)
	}
	if strings.Contains(body, "<p>only html</p>") ||
		!strings.Contains(body, encodeMIMEBody("<p>only html</p>")) {
		t.Errorf("expected base64-encoded HTML body, got:\n%s", body)
	}
	if strings.Contains(body, "multipart/alternative") {
		t.Errorf("unexpected multipart for HTML-only message:\n%s", body)
	}
}

func TestBuildRFC5322MessageSanitizesUntrustedContent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	body, err := buildRFC5322Message("noreply@ra-idp.test", authnports.EmailMessage{
		To:      "bob@example.com",
		Subject: "reset\r\nBcc: attacker@example.com",
		Text:    "line1\rline2\x00",
		HTML:    `<script>alert(1)</script><a href="javascript:alert(1)">x</a>`,
	}, now)
	if err != nil {
		t.Fatalf("buildRFC5322Message: %v", err)
	}
	if strings.Contains(body, "\r\nBcc: attacker@example.com") {
		t.Fatalf("subject created injected header:\n%s", body)
	}
	for _, want := range []string{
		"Subject: reset Bcc: attacker@example.com",
		encodeMIMEBody("line1\r\nline2"),
		encodeMIMEBody(`<script>alert(1)</script><a href="javascript:alert(1)">x</a>`),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing sanitized content %q in body:\n%s", want, body)
		}
	}
	for _, unsafe := range []string{
		"line1\r\nline2",
		"<script>",
		"javascript:alert(1)",
	} {
		if strings.Contains(body, unsafe) {
			t.Errorf("raw unsafe content %q reached SMTP DATA:\n%s", unsafe, body)
		}
	}
	if strings.Contains(body, "\x00") {
		t.Fatalf("body contains NUL byte:\n%s", body)
	}
}

func TestBuildRFC5322MessageRejectsInvalidAddressHeader(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	_, err := buildRFC5322Message("noreply@ra-idp.test", authnports.EmailMessage{
		To: "bob@example.com\r\nBcc: attacker@example.com", Subject: "x", Text: "y",
	}, now)
	if err == nil {
		t.Fatal("buildRFC5322Message should reject injected recipient header")
	}
}

func requireLineContains(t *testing.T, transcript []string, needle string) {
	t.Helper()
	for _, line := range transcript {
		if strings.Contains(line, needle) {
			return
		}
	}
	t.Fatalf("expected %q in transcript:\n%s", needle, strings.Join(transcript, "\n"))
}

func requireBodyContains(t *testing.T, body, needle string) {
	t.Helper()
	if !strings.Contains(body, needle) {
		t.Fatalf("expected %q in body:\n%s", needle, body)
	}
}

func requireBodyNotContains(t *testing.T, body, needle string) {
	t.Helper()
	if strings.Contains(body, needle) {
		t.Fatalf("did not expect %q in body:\n%s", needle, body)
	}
}

// fakeSMTPServer は最小限の SMTP プロトコルを話す。STARTTLS / TLS upgrade は
// しない (本番 TLS パスは ADR-035 § 検証 の手動確認で覆う)。
type fakeSMTPServer struct {
	listener net.Listener
	host     string
	port     int

	mu          sync.Mutex
	transcripts []string
	dataLines   []string
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	server := &fakeSMTPServer{listener: ln, host: addr.IP.String(), port: addr.Port}
	go server.acceptLoop()
	return server
}

func (s *fakeSMTPServer) Close() {
	_ = s.listener.Close()
}

func (s *fakeSMTPServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)
	writeLine(conn, "220 fake-smtp ready")
	inData := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		s.record(line)
		if inData {
			if line == "." {
				inData = false
				writeLine(conn, "250 ok message queued")
				continue
			}
			s.recordDataLine(line)
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			writeLine(conn, "250-fake-smtp hello")
			writeLine(conn, "250 HELP")
		case strings.HasPrefix(upper, "MAIL FROM"), strings.HasPrefix(upper, "RCPT TO"):
			writeLine(conn, "250 ok")
		case strings.HasPrefix(upper, "DATA"):
			inData = true
			writeLine(conn, "354 end data with <CR><LF>.<CR><LF>")
		case strings.HasPrefix(upper, "QUIT"):
			writeLine(conn, "221 bye")
			return
		case strings.HasPrefix(upper, "NOOP"):
			writeLine(conn, "250 ok")
		default:
			writeLine(conn, "500 unrecognized command")
		}
	}
}

func (s *fakeSMTPServer) record(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transcripts = append(s.transcripts, line)
}

func (s *fakeSMTPServer) recordDataLine(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dataLines = append(s.dataLines, line)
}

func (s *fakeSMTPServer) transcript() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.transcripts))
	copy(out, s.transcripts)
	return out
}

func (s *fakeSMTPServer) dataBody() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.dataLines, "\r\n")
}

func writeLine(w net.Conn, line string) {
	_, _ = w.Write([]byte(line + "\r\n"))
}
