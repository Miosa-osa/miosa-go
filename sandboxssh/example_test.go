package sandboxssh_test

import (
	"context"
	"fmt"
	"log"

	miosa "github.com/Miosa-osa/miosa-go/v2"
	"github.com/Miosa-osa/miosa-go/v2/sandboxssh"
)

func ExampleDial() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	conn, err := sandboxssh.Dial(ctx, client, "sbx_1", sandboxssh.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	session, _ := conn.NewSession()
	out, _ := session.Output("uname -a")
	fmt.Println(string(out))
}

func ExampleDialDetailed() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	res, err := sandboxssh.DialDetailed(ctx, client, "sbx_1", sandboxssh.Options{TTLSeconds: 600})
	if err != nil {
		log.Fatal(err)
	}
	res.Client.Close()

	// Reconnect with the same certificate while it is valid.
	again, err := sandboxssh.DialWithCertificate(ctx, client, "sbx_1", res.Key, res.Certificate, sandboxssh.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer again.Close()
}
