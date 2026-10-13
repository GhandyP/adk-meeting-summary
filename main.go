package main

import (
	"context"
	"log"
	"os"

	"github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper"
)

func main() {
	if err := adkhelper.Run(
		context.Background(),
		os.Args[1:],
		"case_04_meeting_transcription_summary",
		"A text-only starting point for transcribe and summarize meetings.",
		"Treat supplied text as a meeting transcript. Summarize decisions, actions, open questions, and preserve any timestamps present; do not claim to transcribe audio.",
	); err != nil {
		log.Fatal(err)
	}
}
