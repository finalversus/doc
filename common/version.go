package common

import (
	"time"
)

const releaseYear = 2019
const releaseMonth = 4
const releaseDay = 20
const releaseHour = 23
const releaseMin = 30

const Version = "3.0.0-rc.1"

var ReleasedAt = time.Date(releaseYear, releaseMonth, releaseDay, releaseHour, releaseMin, 0, 0, time.UTC)
