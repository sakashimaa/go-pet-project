package email

import (
	"context"
	"fmt"
	"net/smtp"
	"os"

	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Sender interface {
	SendActivationEmail(ctx context.Context, to string, token string) error
	SendForgotPasswordEmail(ctx context.Context, to string, token string) error
	SendResetPasswordEmail(ctx context.Context, to string) error
}

type smtpSender struct {
	from     string
	password string
	host     string
	port     string
	logger   *zap.Logger
	tracer   trace.Tracer
}

func NewSMTPSender(logger *zap.Logger) Sender {
	return &smtpSender{
		from:     os.Getenv("SMTP_USER"),
		password: os.Getenv("SMTP_PASSWORD"),
		host:     os.Getenv("SMTP_HOST"),
		port:     os.Getenv("SMTP_PORT"),
		logger:   logger,
		tracer:   otel.Tracer("notification/infrastructure/email"),
	}
}

func (s *smtpSender) SendResetPasswordEmail(ctx context.Context, to string) error {
	ctx, span := s.tracer.Start(ctx, "smtp.SendResetPasswordEmail")
	defer span.End()

	span.SetAttributes(
		attribute.String("to.email", to),
	)

	subject := "Subjet: You recently reset password on our website.\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body := fmt.Sprintf(`
		<h1>If you didnt do it, contact our support</h1>
	`)

	msg := []byte(subject + mime + body)
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	auth := smtp.PlainAuth("", s.from, s.password, s.host)

	mylogger.Info(
		ctx,
		s.logger,
		"Sending reset password email",
		zap.String("to", to),
	)

	if err := smtp.SendMail(addr, auth, s.from, []string{to}, msg); err != nil {
		span.RecordError(err)
		mylogger.Error(
			ctx,
			s.logger,
			"Error sending reset password email",
			zap.String("to", to),
			zap.Error(err),
		)

		return fmt.Errorf("failed to send mail: %v", err)
	}

	mylogger.Info(ctx, s.logger, "Reset password email sent successfully")
	return nil
}

func (s *smtpSender) SendActivationEmail(ctx context.Context, to string, token string) error {
	ctx, span := s.tracer.Start(ctx, "smtp.SendActivationEmail")
	defer span.End()

	span.SetAttributes(
		attribute.String("to.email", to),
		attribute.String("token", token),
	)

	link := fmt.Sprintf("http://localhost:3000/auth/activate?token=%s", token)

	subject := "Subjet: Welcome! Activate your Account.\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body := fmt.Sprintf(`
		<h1>Welcome to our App! 🚀</h1>
		<p>Please click the link below to activate your account:</p>
		<a href="%s">Activate Account</a>
	`, link)

	msg := []byte(subject + mime + body)
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	auth := smtp.PlainAuth("", s.from, s.password, s.host)

	mylogger.Info(
		ctx,
		s.logger,
		"Sending activation email",
		zap.String("to", to),
	)

	if err := smtp.SendMail(addr, auth, s.from, []string{to}, msg); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error sending activation email",
			zap.String("to", to),
			zap.String("token", token),
			zap.Error(err),
		)

		return fmt.Errorf("failed to send mail: %v", err)
	}

	mylogger.Info(
		ctx,
		s.logger,
		"Sent activation email successfully",
		zap.String("to", to),
	)

	return nil
}

func (s *smtpSender) SendForgotPasswordEmail(ctx context.Context, to string, token string) error {
	ctx, span := s.tracer.Start(ctx, "smtp.SendForgotPasswordEmail")
	defer span.End()

	span.SetAttributes(
		attribute.String("to", to),
		attribute.String("token", token),
	)

	link := fmt.Sprintf("http://localhost:3000/auth/reset-password?token=%s", token)

	subject := "Subjet: You requested password reset.\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body := fmt.Sprintf(`
		<h1>Click this link to reset your password</h1>
		<p>If you dont request resetting password, just ignore this message:</p>
		<a href="%s">Reset password</a>
	`, link)

	msg := []byte(subject + mime + body)
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	auth := smtp.PlainAuth("", s.from, s.password, s.host)

	mylogger.Info(
		ctx,
		s.logger,
		"Sending forgot password email",
		zap.String("to", to),
	)

	if err := smtp.SendMail(addr, auth, s.from, []string{to}, msg); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error sending forgot password email",
			zap.String("to", to),
		)

		return fmt.Errorf("failed to send mail: %v", err)
	}

	mylogger.Info(
		ctx,
		s.logger,
		"Sent forgot password email successfully",
		zap.String("to", to),
	)

	return nil
}
