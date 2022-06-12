package formatter

import (
	"fmt"
	"strings"
	"time"
)

func RotaMembersAsString(members []string) string {
	var formattedUserIds []string
	for _, userId := range members {
		formattedUserIds = append(formattedUserIds, fmt.Sprintf("• %s", AtUserId(userId)))
	}
	return fmt.Sprintf("%s", strings.Join(formattedUserIds, "\n"))
}

func AtUserId(userId string) string {
	return fmt.Sprintf("<@%s>", userId)
}

func FormatTime(rawTime time.Time) string {
	return rawTime.Format(time.RFC1123)
}
