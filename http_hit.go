package main

import (
	"strconv"
	"strings"
	"time"
)

type sectionName string

type HTTPHit struct {
	statusCode int
	method     string
	time       time.Time
	bytesSent  int
	section    sectionName
}

func NewHTTPHit(date, method, path, statusCode, bytesSent string) HTTPHit {
	l := HTTPHit{method: method}
	var err error

	l.time, err = time.Parse("02/Jan/2006:15:04:05 -0700", date)
	if err != nil {
		log.Debugf("failed to convert date %q into valid time.Time", date)
		l.time = time.Time{}
	}
	l.statusCode, err = strconv.Atoi(statusCode)
	if err != nil {
		log.Debugf("failed to convert status code %q into integer", statusCode)
		l.statusCode = 0
	}
	l.bytesSent, err = strconv.Atoi(bytesSent)
	if err != nil {
		log.Debugf("failed to convert bytes sent %q into integer", bytesSent)
		l.bytesSent = 0
	}
	if parts := strings.Split(path, "/"); len(parts) <= 2 {
		l.section = "nosection"
	} else {
		l.section = sectionName("/" + parts[1])
	}

	return l
}
