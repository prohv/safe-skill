package proxy

import "log"

func LogIntercept(pkg string, status string, risk int, fileCount int) {
	log.Printf("[%s] %s risk=%d files=%d", pkg, status, risk, fileCount)
}
