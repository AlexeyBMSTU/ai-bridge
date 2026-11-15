package main

import (
    "context"
    "os"

    "github.com/AlexeyBMSTU/ai-bridge/bridge"
)

func main() {
    b := bridge.NewAiBridge(
        os.Getenv("AI_URL"),     
        os.Getenv("API_KEY"),
        "openai/gpt-oss-120b",
        1,
        0,
    )
    b.StartConsoleDialog(context.Background())
}