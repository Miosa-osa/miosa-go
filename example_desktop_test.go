package miosa_test

import (
	"context"
	"fmt"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

// Drive a desktop with typed actions and wait for it first.
func ExampleComputer_Act() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")
	comp, err := client.Computers.Get(ctx, "cmp_123")
	if err != nil {
		return
	}
	if err := comp.WaitForDesktop(ctx, miosa.WaitOptions{Timeout: 90 * time.Second}); err != nil {
		return
	}
	shot, _ := comp.Act(ctx, "screenshot", miosa.ActionParams{})
	fmt.Println(shot.Width, shot.Height)
	click, _ := comp.Act(ctx, "left_click", miosa.ActionParams{Coordinate: []int{512, 384}})
	fmt.Println(click.OK, click.Duration)
}

// Look returns the screenshot and the accessibility tree together.
func ExampleComputer_Look() {
	ctx := context.Background()
	comp, _ := miosa.NewClient("msk_u_...").Computers.Get(ctx, "cmp_123")
	state, err := comp.Look(ctx, miosa.LookOptions{})
	if err != nil {
		return
	}
	fmt.Println(len(state.ImageBase64), len(state.Tree))
}
