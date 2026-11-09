// Command lending serves the member pages of the lending service.
package main

import (
	"log"
	"net/http"
	"os"

	"example.com/lending/loans"
)

func main() {
	service := loans.NewService(loans.Features{OverdueBadges: os.Getenv("LENDING_OVERDUE_BADGES") == "1"}, nil)
	if err := loans.AppendAudit("start"); err != nil {
		log.Printf("audit: %v", err)
	}
	http.HandleFunc("/member", service.MemberPage)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
