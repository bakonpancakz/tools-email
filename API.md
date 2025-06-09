# Introduction
The Engine instance provides a REST API if started using `e.StartHTTP(...)`. 
This interface can be used by applications to append emails to the outbound queue.

- [Introduction](#introduction)
- [📦 Objects](#-objects)
  - [Email](#email)
  - [Address](#address)
  - [Attachment](#attachment)
- [🔗 Endpoints](#-endpoints)
  - [Queue Outbound Emails](#queue-outbound-emails)
    - [Request Body](#request-body)
    - [Responses](#responses)

# 📦 Objects

## Email
The abstract representation of an email.

| Field       | Type                        | Description                                                            |
| ----------- | --------------------------- | ---------------------------------------------------------------------- |
| to          | [Address[]](#address)       | One or more recipients for the email. Must include at least one entry. |
| from        | Address                     | The sender's name and email address.                                   |
| subject     | string                      | The subject line of the email. Max 255 characters.                     |
| content     | string                      | The main message body. HTML or plaintext depending on `html` flag.     |
| html        | boolean                     | Set to `true` if `content` is HTML, `false` if it's plain text.        |
| attachments | [Attachment[]](#attachment) | Optional. One or more file attachments or inline images.               |

## Address
The addresser or recipient of an email.

| Field   | Type   | Description                                                                                                   |
| ------- | ------ | ------------------------------------------------------------------------------------------------------------- |
| name    | string | The display name of the sender or recipient. Must be 1–128 characters.                                        |
| address | string | The email address in [RFC 5322](https://datatracker.ietf.org/doc/html/rfc5322#section-3.4.1) format. Max 128. |

## Attachment
A file attached alongside the email.

| Field        | Type    | Description                                                           |
| ------------ | ------- | --------------------------------------------------------------------- |
| filename     | string  | The name of the attached file. Max 255 characters.                    |
| content_type | string  | The MIME type of the attachment (e.g. `image/png`, `text/plain`).     |
| data         | string  | The base64-encoded binary content of the file.                        |
| inline       | boolean | Set to `true` to embed the attachment inline (e.g. for logos/images). |

> **💡TIP:** Inline attachments like images can be referenced in HTML emails using a Content-ID URL (e.g. `cid:logo.png`)

<br>

# 🔗 Endpoints
All requests to the REST API must contain the following:
- A JSON Encoded Payload with appropriate `Content-Type` Header 
- A Maximum Payload Size of by default 10MB or the custom value in `IncomingMaxBytes`


## Queue Outbound Emails
`POST /queue`

Appends emails to the end of the queue to be later processed and sent.

### Request Body
An array of [Email](#email) Objects
```json
[{
    "to": [{
        "name": "bakonpancakz", 
        "address": "bakonpancakz@gmail.com"
    }],
    "from": {
        "name": "emailengine",
        "address": "emailengine@example.org"
    },
    "html": true,
    "subject": "Setup Complete!",
    "content": "<h1>If you're reading this then your email has been correctly configured! Take this art as your reward!</h1> <img src='cid:teto.png' alt='Kasane Teto'/>",
    "attachments": [{
        "content_type": "image/png",
        "filename": "teto.png",
        "data": "iVBORw0KGgoAAAANSUhEUgAAAMgAAADgBAMAAAC0iTT2AAAAKlBMVEUAAAAAAADVACCIABX87dHDw8Pw8PBPT0/v26/LX19/f3/tHCT/rsmGKysrGzxDAAAAAXRSTlMAQObYZgAAAadJREFUeNrt2kFxwzAQheFSCAVTMAVTCAVTKIVSKIVQCIVSKJfuTvWmbxTZTa6r/102kqP9fNlx3cnbq7lEXr0GAjITkk3ukay+p7pHsoKAzIqMADVWBQGZFbl0WSK+FgACUh25DKL95YkIGvUBAamAHDXSQ+qzJfey9mt9PusDAlIRWSNqsjwRfU/nlD0CAlIFWQ9yj+jg8Ea8tuwWEJAqSGQI6aA37ZEjSBEAAlIF6Rv0wEckq6/7vf5GQECqIasyAL4jfcMtMtrfLSAglZAjSEOXDbKpGmbNtfZH5wSAgFRDfADvEQ1dft5aBPj+6I9BEJCqiA+S1l8t2yC65mf2FhCQioggHyQd3k6im/Eb1XkQkIqIvviwd5Kj8yAglZH/BlUvP2fNQEBmRLLhLXKNrJGsuc59EBCQX+Da8h7xmsnrICAzI3qp0SAK8IHUSw8IyMyIHlQCtPYKAjIrokH0Iby1LBEfUA0kCMiMyBbxh5TizfP6FgEBmRERlBHkEeD/hAYBmRFRBPXx5iAgIH8/BvA1CAjI4zBm9TUICIghFl+DgEyC/AAl2AjG7TSPSgAAAABJRU5ErkJggg==",
        "inline": true
    }]
}]
```

### Responses
| Code                           | Meaning                                                           |
| :----------------------------- | :---------------------------------------------------------------- |
| `413 Request Entity Too Large` | Request Payload is Too Large                                      |
| `401 Unauthorized`             | The AuthHandler rejected the incoming request                     |
| `415 Unsupported Media Type`   | Request Header `Content-Type` does not equal `application/json`   |
| `422 Unprocessable Entity`     | Request Payload is a invalid or malformed JSON string             |
| `400 Bad Request`              | Some Emails have failed validation and were rejected              |
| `507 Insufficient Storage`     | Some Emails could not fit in the internal queue and were rejected |
| `201 Created`                  | Provided Emails were succesfully queued                           |
