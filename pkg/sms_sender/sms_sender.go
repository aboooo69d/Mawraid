package smsSender

// this file is now empty for test dont test it
type Provider interface {
	SendSMS(to string, msg string) error
}

type HttpSmsClient struct {
	BaseUrl string
	ApiKey  string
}

func NewSmsService() Provider {
	return &HttpSmsClient{
		BaseUrl: "",
		ApiKey:  "",
	}
}

func (s *HttpSmsClient) SendSMS(to string, msg string) error {
	return nil
}
