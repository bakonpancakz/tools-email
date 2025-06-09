package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"
)

var (
	CONTEXT_TIMEOUT  = 10 * time.Second
	SMTP_DOMAIN      = envString("SMTP_DOMAIN", "example.org")
	HTTP_ADDRESS     = envString("HTTP_ADDRESS", "http://localhost:80/queue")
	SMTP_DESTINATION = envString("SMTP_DESTINATION", "user@example.org")
)

var (
	//go:embed templates/*.html
	templateFS embed.FS
	//go:embed logo.png
	templateLogo []byte
)

func main() {
	// Prepare Template
	// 	Preferably you'd have a lot of these sitting at the top of a file somwhere
	type LocalsForgotPassword struct {
		Displayname string
		Token       string
	}
	template := loadTemplate[LocalsForgotPassword]("FORGOT_PASSWORD", "Password Reset Request")

	// You application would then query some data and execute a template
	// sending the rendered output to the engines REST API
	outboundAddress := SMTP_DESTINATION
	outboundLocals := LocalsForgotPassword{
		Displayname: "Example User",
		Token:       "example-token",
	}
	if err := template(outboundAddress, outboundLocals); err != nil {
		log.Fatalln("Cannot Send Email:", err)
	}
}

// Prepare a template returning a helper function which can called in the future
// to execute and queue the function in the future.
func loadTemplate[L any](filename, subjectLine string) func(emailAddress string, locals L) error {
	template, err := template.ParseFS(
		templateFS,
		"templates/TEMPLATE.html",
		"templates/"+filename+".html",
	)
	if err != nil {
		log.Fatalln("Cannot Parse Template:", err)
	}
	return func(emailAddress string, locals L) error {
		// Generate Payload
		output := bytes.Buffer{}
		if err := template.Execute(&output, locals); err != nil {
			return err
		}
		payload, err := json.Marshal([]map[string]any{{
			"to": []map[string]any{{
				"name":    emailAddress,
				"address": emailAddress,
			}},
			"from": map[string]any{
				"name":    "Example Inc.",
				"address": "noreply@" + SMTP_DOMAIN,
			},
			"subject": subjectLine,
			"content": output.String(),
			"html":    true,
			"attachments": []map[string]any{{
				"content_type": "image/png",
				"filename":     "logo.png",
				"data":         templateLogo,
				"inline":       true,
			}},
		}})
		if err != nil {
			return err
		}

		// Generate Request
		ctx, cancel := context.WithTimeout(context.Background(), CONTEXT_TIMEOUT)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, "POST", HTTP_ADDRESS, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		// Validate Response
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(response.Body)
			return fmt.Errorf("server responded with status %d: %s", response.StatusCode, string(body))
		}

		// Log Outbound Email
		log.Printf("Outgoing Email: %s => %s\n", filename, emailAddress)
		return nil
	}
}

func envString(field, initial string) string {
	var Value = os.Getenv(field)
	if Value == "" {
		if initial == "\x00" {
			fmt.Printf("Variable '%s' was not set\n", field)
			os.Exit(2)
		}
		return initial
	}
	return Value
}
