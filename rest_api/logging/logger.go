package logging

type Logger interface {
	Trace(msg string)
	Debug(msg string)
	Info(msg string)
	Error(msg string)
	Fatal(msg string)
}
