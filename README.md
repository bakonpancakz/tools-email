# 📧 `tools-email`
A lightweight REST + SMTP service for sending and forwarding emails, with DKIM signing and SSL support.

* Send HTML or plaintext emails via a simple REST API.
* Automatically forward incoming emails to your personal inbox.
* Built-in support for DMARC and DKIM.
* No external dependencies—self-hosted and secure.
* Helpful Alert when emails are sent to the noreply inbox.

> **NOTE:** DMARC Emails are not processed and silently dropped.
> This helps prevents the server from getting marked as spam by faking compliance.

---

## ⚙️ Configuration
To run the service, you’ll need a valid SSL certificate and DKIM RSA key. You can set these using environment variables or a `.env` file in the working directory.
Required Variables are marked with an asterisk `*`.

| Environment Variable     | Default          | Description                                                                                            |
| ------------------------ | ---------------- | ------------------------------------------------------------------------------------------------------ |
| `*TLS_CERT`              | `tls_crt.pem`    | Path to your SSL certificate                                                                           |
| `*TLS_KEY`               | `tls_key.pem`    | Path to your SSL private key                                                                           |
| `*TLS_CA`                | `tls_ca.pem`     | Path to your certificate authority file                                                                |
| `*DKIM_KEY`              | `dkim.pem`       | Path to your DKIM RSA key                                                                              |
| `*HTTP_PASSPHRASE`       | `teto`           | Passphrase required for REST API authentication                                                        |
| `*HTTP_ADDRESS`          | `localhost:8800` | Address and port the HTTP server listens on                                                            |
| `*SMTP_ADDRESS`          | `localhost:2525` | Address and port the SMTP server listens on                                                            |
| `*SMTP_DOMAIN`           | `example.org`    | Domain name used for outgoing SMTP messages                                                            |
| `*SMTP_USERNAME_DMARC`   | `dmarc`          | Username used to receive DMARC-related emails                                                          |
| `*SMTP_USERNAME_NOREPLY` | `noreply`        | Username for outgoing emails                                                                           |
| `*SMTP_USERNAME_FORWARD` | `support`        | Username for receiving incoming mail                                                                   |
| `SMTP_FORWARD_ADDRESS`   | *(empty)*        | Email address to forward incoming mail to (leave blank to disable forwarding)                          |
| `SMTP_DISABLE_NOREPLY`   | *(empty)*          | Set this variable to any value to disable the alert when sending an email to the noreply inbox |

---

## 📬 Sending an Email
To send an email, make a `POST` request to the root endpoint `/` with your `HTTP_PASSPHRASE` in the `Authorization` header. 
The request body should be a JSON object with the email content:

### Request Example

```json
POST / HTTP/1.1
Authorization: teto
Content-Type: application/json

{
    "to_name": "bakonpancakz@gmail.com",
    "to_address": "bakonpancakz@gmail.com",
    "from_name": "Support",
    "from_address": "support@example.org",
    "subject": "Hello World!",
    "content": "<h1>Service successfully setup! Now enjoy your image!</h1> <img src='cid:teto.png' alt='Kasane Teto!'/>",
    "html": true,
    "attachments": [{
        "content_type": "image/png",
        "filename": "teto.png",
        "data": "iVBORw0KGgoAAAANSUhEUgAAAMgAAADgBAMAAAC0iTT2AAAAKlBMVEUAAAAAAADVACCIABX87dHDw8Pw8PBPT0/v26/LX19/f3/tHCT/rsmGKysrGzxDAAAAAXRSTlMAQObYZgAAAadJREFUeNrt2kFxwzAQheFSCAVTMAVTCAVTKIVSKIVQCIVSKJfuTvWmbxTZTa6r/102kqP9fNlx3cnbq7lEXr0GAjITkk3ukay+p7pHsoKAzIqMADVWBQGZFbl0WSK+FgACUh25DKL95YkIGvUBAamAHDXSQ+qzJfey9mt9PusDAlIRWSNqsjwRfU/nlD0CAlIFWQ9yj+jg8Ea8tuwWEJAqSGQI6aA37ZEjSBEAAlIF6Rv0wEckq6/7vf5GQECqIasyAL4jfcMtMtrfLSAglZAjSEOXDbKpGmbNtfZH5wSAgFRDfADvEQ1dft5aBPj+6I9BEJCqiA+S1l8t2yC65mf2FhCQioggHyQd3k6im/Eb1XkQkIqIvviwd5Kj8yAglZH/BlUvP2fNQEBmRLLhLXKNrJGsuc59EBCQX+Da8h7xmsnrICAzI3qp0SAK8IHUSw8IyMyIHlQCtPYKAjIrokH0Iby1LBEfUA0kCMiMyBbxh5TizfP6FgEBmRERlBHkEeD/hAYBmRFRBPXx5iAgIH8/BvA1CAjI4zBm9TUICIghFl+DgEyC/AAl2AjG7TSPSgAAAABJRU5ErkJggg==",
        "inline": true
    }]
}
```