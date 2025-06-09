package email

type Address struct {
	Name    string `validate:"required,min=1,max=128" json:"name"`
	Address string `validate:"required,email,max=128" json:"address"`
}

type Attachment struct {
	ContentType string `validate:"required,max=255" json:"content_type"`
	Filename    string `validate:"required,max=255" json:"filename"`
	Data        []byte `validate:"required" json:"data"`
	Inline      bool   `validate:"required" json:"inline"`
}

type Email struct {
	To          []Address    `validate:"required,dive" json:"to"`
	From        Address      `validate:"required" json:"from"`
	Subject     string       `validate:"required" json:"subject"`
	Content     string       `validate:"required" json:"content"`
	HTML        bool         `validate:"required" json:"html"`
	Attachments []Attachment `validate:"dive" json:"attachments"`
}
