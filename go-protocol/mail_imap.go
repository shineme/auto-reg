package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var codePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Your verification code is:\s*(\d{6})\b`),
	regexp.MustCompile(`(?i)verification code[:\s]*(\d{6})\b`),
	regexp.MustCompile(`(?i)(?:code|验证码)[^\d]{0,12}(\d{6})\b`),
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func extractVerificationCodes(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, re := range codePatterns {
		matches := re.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) > 1 && !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	return out
}

func cleanMailBody(raw string) string {
	if decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(raw))); err == nil && len(decoded) > 0 {
		raw = string(decoded)
	}
	raw = htmlTagRe.ReplaceAllString(raw, " ")
	return strings.Join(strings.Fields(raw), " ")
}

func PollIMAPCode(ctx context.Context, host string, port int, user, pass, recipient string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	recipientLower := strings.ToLower(strings.TrimSpace(recipient))
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		code, err := searchIMAPOnce(ctx, host, port, user, pass, recipientLower)
		if err == nil && code != "" {
			return code, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(4 * time.Second):
		}
	}
	return "", fmt.Errorf("IMAP 轮询超时，未收到 %s 的验证码", recipient)
}

func searchIMAPOnce(ctx context.Context, host string, port int, user, pass, recipientLower string) (string, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	dialer := &tls.Dialer{Config: &tls.Config{ServerName: host}}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))

	r := bufio.NewReader(conn)
	if _, err := r.ReadString('\n'); err != nil {
		return "", err
	}

	seq := 0
	cmd := func(line string) (string, error) {
		seq++
		tag := fmt.Sprintf("A%04d", seq)
		if _, err := fmt.Fprintf(conn, "%s %s\r\n", tag, line); err != nil {
			return "", err
		}
		var sb strings.Builder
		for {
			l, err := r.ReadString('\n')
			sb.WriteString(l)
			if err != nil {
				return sb.String(), err
			}
			if strings.HasPrefix(l, tag+" ") {
				if !strings.HasPrefix(l, tag+" OK") {
					return sb.String(), fmt.Errorf("IMAP 命令失败: %s", strings.TrimSpace(l))
				}
				return sb.String(), nil
			}
		}
	}

	if _, err := cmd(fmt.Sprintf(`LOGIN %q %q`, user, pass)); err != nil {
		return "", err
	}
	defer func() { _, _ = cmd("LOGOUT") }()
	if _, err := cmd(`SELECT "INBOX"`); err != nil {
		return "", err
	}

	since := time.Now().Add(-24 * time.Hour).Format("02-Jan-2006")
	searchResp, err := cmd(fmt.Sprintf(`SEARCH SINCE %s`, since))
	if err != nil {
		return "", err
	}
	var ids []int
	for _, line := range strings.Split(searchResp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "* SEARCH") {
			fields := strings.Fields(line)[2:]
			for _, f := range fields {
				if id, err := strconv.Atoi(f); err == nil {
					ids = append(ids, id)
				}
			}
		}
	}
	if len(ids) == 0 {
		return "", errors.New("no recent mail")
	}
	// Check latest 25 messages newest first
	start := len(ids) - 25
	if start < 0 {
		start = 0
	}
	for i := len(ids) - 1; i >= start; i-- {
		id := ids[i]
		fetchResp, err := cmd(fmt.Sprintf(`FETCH %d (BODY.PEEK[HEADER.FIELDS (TO SUBJECT)] BODY.PEEK[TEXT])`, id))
		if err != nil {
			continue
		}
		lower := strings.ToLower(fetchResp)
		if recipientLower != "" && !strings.Contains(lower, recipientLower) {
			continue
		}
		dec := decodeHeaderOrBody(fetchResp)
		codes := extractVerificationCodes(cleanMailBody(dec))
		if len(codes) > 0 {
			return codes[0], nil
		}
	}
	return "", errors.New("code not found yet")
}

func decodeHeaderOrBody(s string) string {
	dec := new(mime.WordDecoder)
	if out, err := dec.DecodeHeader(s); err == nil {
		s = out
	}
	// Fallback base64 line check if needed
	_ = base64.StdEncoding
	return s
}
