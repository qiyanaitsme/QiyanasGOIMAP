package main

import (
	"bufio"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/fatih/color"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type EmailMessage struct {
	From    string
	Subject string
	Date    time.Time
	Body    string
	SeqNum  uint32
}

func printMenu() {
	color.HiCyan(`
╔══════════════════════════════════════════════════╗
║             QIYANA CODE GRABBER                  ║
║                                                  ║
║   ██████╗ ██╗██╗   ██╗ █████╗ ███╗   ██╗ █████╗  ║
║  ██╔═══██╗██║╚██╗ ██╔╝██╔══██╗████╗  ██║██╔══██╗ ║
║  ██║   ██║██║ ╚████╔╝ ███████║██╔██╗ ██║███████║ ║
║  ██║▄▄ ██║██║  ╚██╔╝  ██╔══██║██║╚██╗██║██╔══██║ ║
║  ╚██████╔╝██║   ██║   ██║  ██║██║ ╚████║██║  ██║ ║
║   ╚══▀▀═╝ ╚═╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝ ║
║                                                  ║
║  Created by QIYANA                               ║
║  https://lolz.live/kqlol/                        ║
║  Version: 2.0                                    ║
║                                                  ║
╚══════════════════════════════════════════════════╝
`)
}

func printStatus(status string) {
	color.HiYellow(`
╔══════════════════════════════════════════════════╗
║                    СТАТУС                        ║
╠══════════════════════════════════════════════════╣
║ %s
╚══════════════════════════════════════════════════╝
`, status)
}

func logInfo(format string, args ...interface{}) {
	prefix := color.GreenString("[INFO] ")
	timestamp := time.Now().Format("15:04:05")
	timeStr := color.HiBlackString("[%s] ", timestamp)
	fmt.Printf(timeStr+prefix+format+"\n", args...)
}

func logError(format string, args ...interface{}) {
	prefix := color.RedString("[ERROR] ")
	timestamp := time.Now().Format("15:04:05")
	timeStr := color.HiBlackString("[%s] ", timestamp)
	fmt.Printf(timeStr+prefix+format+"\n", args...)
}

func getCredentials() (string, string) {
	reader := bufio.NewReader(os.Stdin)
	color.Yellow("\n[*] Введите учетные данные (email:password): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	credentials := strings.Split(input, ":")
	if len(credentials) != 2 {
		logError("Неверный формат. Используйте email:password")
		os.Exit(1)
	}

	return credentials[0], credentials[1]
}

func connectToIMAP(email, password string) *client.Client {
	logInfo("Подключение к IMAP серверу...")
	c, err := client.DialTLS("imap.firstmail.ltd:993", nil)
	if err != nil {
		logError("Ошибка подключения: %v", err)
		os.Exit(1)
	}

	logInfo("Выполняется вход в почтовый ящик...")
	if err := c.Login(email, password); err != nil {
		logError("Ошибка входа: %v", err)
		os.Exit(1)
	}
	logInfo("Успешный вход в систему!")

	return c
}

func fetchLastEmails(c *client.Client, mbox *imap.MailboxStatus, count uint32) []EmailMessage {
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > count {
		from = mbox.Messages - count + 1
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddRange(from, to)

	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}

	messages := make(chan *imap.Message, count)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqSet, items, messages)
	}()

	var emailMessages []EmailMessage
	for msg := range messages {
		r := msg.GetBody(section)
		if r == nil {
			continue
		}

		var bodyBuilder strings.Builder
		_, err := io.Copy(&bodyBuilder, r)
		if err != nil {
			continue
		}

		emailMsg := EmailMessage{
			From:    msg.Envelope.From[0].PersonalName,
			Subject: msg.Envelope.Subject,
			Date:    msg.Envelope.Date,
			Body:    bodyBuilder.String(),
			SeqNum:  msg.SeqNum,
		}
		emailMessages = append(emailMessages, emailMsg)
	}

	if err := <-done; err != nil {
		logError("Ошибка получения писем: %v", err)
		os.Exit(1)
	}

	return emailMessages
}

func extractCode(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		cleanLine := strings.TrimSpace(line)
		if matched, _ := regexp.MatchString(`^\d{6}$`, cleanLine); matched {
			return cleanLine
		}
	}
	return "Код не найден"
}

func displaySelectedEmail(msg EmailMessage) {
	code := extractCode(msg.Body)
	if code != "Код не найден" {
		color.HiCyan("\n╔══════════════════╗")
		color.HiCyan("║     Найден код     ║")
		color.HiCyan("║      %s            ║", code)
		color.HiCyan("╚════════════════════╝")

		clipboard.WriteAll(code)
		color.Green("Код скопирован в буфер обмена!")
	} else {
		color.Red("\nКод в письме не обнаружен")
	}
}

func findEmailBySubject(messages []EmailMessage, subject string) *EmailMessage {
	for _, msg := range messages {
		if strings.Contains(msg.Subject, subject) {
			return &msg
		}
	}
	return nil
}

func clear() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func main() {
	for {
		clear()
		printMenu()
		email, password := getCredentials()

		printStatus("Подключение к серверу...")
		c := connectToIMAP(email, password)

		printStatus("Поиск письма...")
		mbox, err := c.Select("INBOX", false)
		if err != nil {
			logError("Ошибка выбора INBOX: %v", err)
			c.Logout()
			continue
		}

		messages := fetchLastEmails(c, mbox, 10)
		targetSubject := "Epic Games - Email Verification"

		if foundEmail := findEmailBySubject(messages, targetSubject); foundEmail != nil {
			displaySelectedEmail(*foundEmail)
		} else {
			logError("Письмо с темой '%s' не найдено", targetSubject)
		}

		c.Logout()

		color.HiMagenta("\n⏳ Автоматический перезапуск через 5 секунд...")
		time.Sleep(5 * time.Second)
	}
}
