package publisher

type Message interface {
	Topic() string
	Payload() interface{}
	ContentType() string
}

type message struct {
	topic       string
	payload     interface{}
	contentType string
}

func (m *message) Topic() string {
	return m.topic
}

func (m *message) Payload() interface{} {
	return m.payload
}

func (m *message) ContentType() string {
	return m.contentType
}

// NewMessage creates a new business-level message
func NewMessage(topic string, payload interface{}, contentType string) Message {
	return &message{
		topic:       topic,
		payload:     payload,
		contentType: contentType,
	}
}
