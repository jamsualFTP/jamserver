package ansi

import (
	"fmt"
)

func SetScroll() string {
	// WARNING: termporary
	CURSOR_UP_ONE := "\x1b[1A"
	ERASE_LINE := "\x1b[2K"

	return fmt.Sprint(CURSOR_UP_ONE, ERASE_LINE)
}
