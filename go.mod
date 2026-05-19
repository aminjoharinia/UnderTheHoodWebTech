module framework-detector

go 1.22

require (
	golang.org/x/net v0.26.0
	gopkg.in/telebot.v4 v4.0.0-beta.8
)

require github.com/joho/godotenv v1.5.1

// GOOS=linux   GOARCH=amd64 garble -literals -tiny build -o UnderTheHoodWeb_linux_amd64   main.go
// GOOS=linux   GOARCH=arm64 garble -literals -tiny build -o UnderTheHoodWeb_linux_arm64   main.go
// GOOS=windows GOARCH=amd64 garble -literals -tiny build -o UnderTheHoodWeb_Windows_amd64.exe main.go
// GOOS=darwin  GOARCH=arm64 garble -literals -tiny build -o UnderTheHoodWeb_MacOS_arm64     main.go
