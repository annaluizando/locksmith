package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"token-toolkit/deployment"
	secrets "token-toolkit/jwt-rotation"
	"token-toolkit/jwt-rotation/notifiers"
	"token-toolkit/jwt-rotation/storage"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const locksmithBanner = `
█  ▄▄▄  ▗▞▀▘█  ▄  ▄▄▄ ▄▄▄▄  ▄    ■  ▐▌   
█ █   █ ▝▚▄▖█▄▀  ▀▄▄  █ █ █ ▄ ▗▄▟▙▄▖▐▌   
█ ▀▄▄▄▀     █ ▀▄ ▄▄▄▀ █   █ █   ▐▌  ▐▛▀▚▖
█           █  █            █   ▐▌  ▐▌ ▐▌
                                ▐▌       
`

// Styles holds the lipgloss styles for the UI.
type Styles struct {
	App      lipgloss.Style
	Title    lipgloss.Style
	Choice   lipgloss.Style
	Selected lipgloss.Style
	Info     lipgloss.Style
	Error    lipgloss.Style
}

func defaultStyles() *Styles {
	s := new(Styles)
	s.App = lipgloss.NewStyle().Padding(1, 2)
	s.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(0, 0, 1, 0)
	s.Choice = lipgloss.NewStyle().PaddingLeft(2)
	s.Selected = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("205")).SetString("> ")
	s.Info = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.Error = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	return s
}

type model struct {
	providerChoices   []string
	cursor            int
	state             appState
	configInputs      []textinput.Model
	provider          string
	executionMode     executionMode
	notifierChoices   []string
	selectedNotifiers map[int]struct{}
	spinner           spinner.Model
	styles            *Styles
	message           string
	initialAction     initialAction
}

type appState int
type executionMode int
type initialAction int

const (
	choosingAction appState = iota
	choosingProvider
	enteringConfig
	choosingNotifier
	choosingMode
	generatingScript
	rotating
	done
	appError
)

const (
	runOnce executionMode = iota
	runPeriodic
)

const (
	actionRotate initialAction = iota
	actionCheckStatus
)

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		providerChoices:   []string{"GCP", "AWS", "Azure"},
		state:             choosingAction,
		notifierChoices:   []string{"Sentry", "Slack"},
		selectedNotifiers: make(map[int]struct{}),
		spinner:           s,
		styles:            defaultStyles(),
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case choosingAction:
			return updateChoosingAction(msg, m)
		case choosingProvider:
			return updateChoosingProvider(msg, m)
		case enteringConfig:
			return updateEnteringConfig(msg, m)
		case choosingNotifier:
			return updateChoosingNotifier(msg, m)
		case choosingMode:
			return updateChoosingMode(msg, m)
		case done, appError:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			}
		}
	case *rotationMsg:
		m.state = done
		m.message = "Secret rotated successfully!"
		return m, tea.Quit
	case *rotationErrMsg:
		m.state = appError
		m.message = "Error during rotation: " + msg.err.Error()
		return m, tea.Quit
	case *rotationStartedMsg:
		m.state = rotating
		return m, nil
	case *scriptGeneratedMsg:
		m.state = done
		m.message = "Deployment script generated: " + msg.filename
		return m, tea.Quit
	case *statusMsg:
		m.state = done
		m.message = fmt.Sprintf("Last rotation: %s", msg.lastRotated.Format(time.RFC3339))
		return m, tea.Quit
	}

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func updateChoosingAction(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter":
		m.initialAction = initialAction(m.cursor)

		m.state = choosingProvider
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func updateChoosingProvider(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.providerChoices)-1 {
			m.cursor++
		}
	case "enter":
		m.provider = m.providerChoices[m.cursor]
		m.state = enteringConfig
		m.cursor = 0
		m.configInputs = setupConfigInputs(m.provider)
		return m, m.configInputs[0].Focus()
	}
	return m, nil
}

func updateEnteringConfig(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter":
		if m.cursor == len(m.configInputs) {
			if m.initialAction == actionCheckStatus {
				m.state = rotating // we can reuse this state to show a spinner
				return m, checkStatus(m)
			}
			m.state = choosingNotifier
			m.cursor = 0
			return m, nil
		}
		if m.cursor < len(m.configInputs)-1 {
			m.cursor++
			cmd = m.configInputs[m.cursor].Focus()
			cmds = append(cmds, cmd)
		} else {
			m.cursor++ // Move to submit
		}

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < len(m.configInputs) {
				cmd = m.configInputs[m.cursor].Focus()
				cmds = append(cmds, cmd)
			}
		}
	case "down", "j":
		if m.cursor < len(m.configInputs) {
			m.cursor++
			if m.cursor < len(m.configInputs) {
				cmd = m.configInputs[m.cursor].Focus()
				cmds = append(cmds, cmd)
			}
		}
	}

	for i := range m.configInputs {
		m.configInputs[i], cmd = m.configInputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func updateChoosingNotifier(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.notifierChoices) { // +1 for the done button
			m.cursor++
		}
	case " ":
		if _, ok := m.selectedNotifiers[m.cursor]; ok {
			delete(m.selectedNotifiers, m.cursor)
		} else {
			m.selectedNotifiers[m.cursor] = struct{}{}
		}
	case "enter":
		m.state = choosingMode
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func updateChoosingMode(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter":
		if m.cursor == 0 {
			m.executionMode = runOnce
			m.state = rotating
			return m, tea.Batch(runRotation(m), m.spinner.Tick)
		} else {
			m.executionMode = runPeriodic
			m.state = generatingScript
			return m, generateScriptCmd(m)
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(m.styles.Title.Render(locksmithBanner))
	b.WriteString("\n")

	switch m.state {
	case choosingAction:
		b.WriteString(m.styles.Title.Render("What would you like to do?"))
		b.WriteString("\n")
		actions := []string{"Rotate Secrets", "Check Status"}
		for i, action := range actions {
			if m.cursor == i {
				b.WriteString(m.styles.Selected.Render(action))
			} else {
				b.WriteString(m.styles.Choice.Render(action))
			}
			b.WriteString("\n")
		}
	case choosingProvider:
		b.WriteString(m.styles.Title.Render("Select the cloud provider:"))
		b.WriteString("\n")
		for i, choice := range m.providerChoices {
			if m.cursor == i {
				b.WriteString(m.styles.Selected.Render(choice))
			} else {
				b.WriteString(m.styles.Choice.Render(choice))
			}
			b.WriteString("\n")
		}
	case enteringConfig:
		b.WriteString(m.styles.Title.Render(fmt.Sprintf("Enter configuration for %s:", m.provider)))
		b.WriteString("\n")
		for i, input := range m.configInputs {
			b.WriteString(input.View())
			if m.cursor == i {
				b.WriteString(" <")
			}
			b.WriteString("\n")
		}

		submit := "[Submit]"
		if m.cursor == len(m.configInputs) {
			submit = m.styles.Selected.Render("[Submit]")
		}
		b.WriteString("\n" + submit + "\n")

	case choosingNotifier:
		b.WriteString(m.styles.Title.Render("Select notification channels (space to select, enter to continue):"))
		b.WriteString("\n")
		for i, choice := range m.notifierChoices {
			selected := " "
			if _, ok := m.selectedNotifiers[i]; ok {
				selected = "x"
			}
			line := fmt.Sprintf("[%s] %s", selected, choice)
			if m.cursor == i {
				b.WriteString(m.styles.Selected.Render(line))
			} else {
				b.WriteString(m.styles.Choice.Render(line))
			}
			b.WriteString("\n")
		}

		doneButton := "[Done]"
		if m.cursor == len(m.notifierChoices) {
			doneButton = m.styles.Selected.Render("[Done]")
		}
		b.WriteString("\n" + doneButton + "\n")

	case choosingMode:
		b.WriteString(m.styles.Title.Render("How do you want to run the rotation?"))
		b.WriteString("\n\n")
		choices := []string{"Run once", "Run periodically (deploy to cloud)"}
		for i, choice := range choices {
			if m.cursor == i {
				b.WriteString(m.styles.Selected.Render(choice))
			} else {
				b.WriteString(m.styles.Choice.Render(choice))
			}
			b.WriteString("\n")
		}
	case generatingScript:
		b.WriteString(fmt.Sprintf("%s Generating deployment script...", m.spinner.View()))
	case rotating:
		b.WriteString(fmt.Sprintf("%s Rotating secret...", m.spinner.View()))
	case done:
		b.WriteString(m.styles.Title.Render(m.message))
	case appError:
		b.WriteString(m.styles.Error.Render(m.message))
	}

	b.WriteString(m.styles.Info.Render("\nPress 'q' or 'ctrl+c' to quit.\n"))
	return m.styles.App.Render(b.String())
}

func setupConfigInputs(provider string) []textinput.Model {
	var inputs []textinput.Model
	switch provider {
	case "GCP":
		inputs = make([]textinput.Model, 2)
		inputs[0] = textinput.New()
		inputs[0].Placeholder = "Project ID"
		inputs[0].Focus()
		inputs[1] = textinput.New()
		inputs[1].Placeholder = "Secret ID"
	case "AWS":
		inputs = make([]textinput.Model, 2)
		inputs[0] = textinput.New()
		inputs[0].Placeholder = "Secret ID"
		inputs[0].Focus()
		inputs[1] = textinput.New()
		inputs[1].Placeholder = "Region"
	case "Azure":
		inputs = make([]textinput.Model, 2)
		inputs[0] = textinput.New()
		inputs[0].Placeholder = "Vault URI"
		inputs[0].Focus()
		inputs[1] = textinput.New()
		inputs[1].Placeholder = "Secret Name"
	}
	return inputs
}

func runRotation(m model) tea.Cmd {
	return func() tea.Msg {
		config := make(map[string]string)
		for _, input := range m.configInputs {
			config[strings.ToLower(strings.ReplaceAll(input.Placeholder, " ", ""))] = input.Value()
		}

		var storageProvider storage.SecretStorage
		switch m.provider {
		case "GCP":
			storageProvider = storage.NewGCPSecretManager()
		case "AWS":
			storageProvider = storage.NewAWSSecretsManager()
		case "Azure":
			storageProvider = storage.NewAzureKeyVault()
		}

		ctx := context.Background()
		if err := storageProvider.Setup(ctx, config); err != nil {
			log.Printf("Error setting up storage: %v", err)
			return &rotationErrMsg{err}
		}

		policy := secrets.RotationPolicy{
			RotationInterval: 0,
			GracePeriod:      48 * time.Hour,
		}

		var notifiersList []secrets.Notifier
		for i := range m.selectedNotifiers {
			switch m.notifierChoices[i] {
			case "Sentry":
				sentryNotifier, err := notifiers.NewSentryNotifier()
				if err != nil {
					log.Printf("Failed to create sentry notifier: %v", err)
				} else if sentryNotifier != nil {
					notifiersList = append(notifiersList, sentryNotifier)
				}
			case "Slack":
				slackNotifier, err := notifiers.NewSlackNotifier()
				if err != nil {
					log.Printf("Failed to create slack notifier: %v", err)
				} else if slackNotifier != nil {
					notifiersList = append(notifiersList, slackNotifier)
				}
			}
		}

		notifier := notifiers.NewMultiNotifier(notifiersList...)

		secretManager, err := secrets.NewJWTManager(policy, 64, storageProvider, notifier)
		if err != nil {
			log.Printf("Failed to create secret manager: %v", err)
			return &rotationErrMsg{err}
		}

		if _, err := secretManager.RotateSecret(); err != nil {
			log.Printf("Failed to rotate secret: %v", err)
			return &rotationErrMsg{err}
		}
		return &rotationMsg{}
	}
}

func checkStatus(m model) tea.Cmd {
	return func() tea.Msg {
		if m.provider == "" {
			return &rotationErrMsg{fmt.Errorf("provider not selected")}
		}

		config := make(map[string]string)
		for _, input := range m.configInputs {
			config[strings.ToLower(strings.ReplaceAll(input.Placeholder, " ", ""))] = input.Value()
		}

		var storageProvider storage.SecretStorage
		switch m.provider {
		case "GCP":
			storageProvider = storage.NewGCPSecretManager()
		case "AWS":
			storageProvider = storage.NewAWSSecretsManager()
		case "Azure":
			storageProvider = storage.NewAzureKeyVault()
		default:
			return &rotationErrMsg{fmt.Errorf("unsupported provider: %s", m.provider)}
		}

		if storageProvider == nil {
			return &rotationErrMsg{fmt.Errorf("failed to create storage provider for %s", m.provider)}
		}

		ctx := context.Background()
		if err := storageProvider.Setup(ctx, config); err != nil {
			return &rotationErrMsg{err}
		}

		latestSecret, err := storageProvider.GetLatest(ctx)
		if err != nil {
			return &rotationErrMsg{err}
		}

		return &statusMsg{
			lastRotated: latestSecret.CreatedAt,
		}
	}
}

func generateScriptCmd(m model) tea.Cmd {
	return func() tea.Msg {
		config := make(map[string]string)
		for _, input := range m.configInputs {
			key := strings.ToLower(strings.ReplaceAll(input.Placeholder, " ", ""))
			config[key] = input.Value()
		}

		data := deployment.ScriptData{
			Provider:       m.provider,
			SecretID:       config["secretid"],
			ProjectID:      config["projectid"],
			Region:         config["region"],
			VaultURI:       config["vaulturi"],
			SecretName:     config["secretname"],
			SentryDSN:      os.Getenv("SENTRY_DSN"),
			SlackBotToken:  os.Getenv("SLACK_BOT_TOKEN"),
			SlackChannelID: os.Getenv("SLACK_CHANNEL_ID"),
		}

		script, err := deployment.GenerateScript(data)
		if err != nil {
			return &rotationErrMsg{err}
		}

		filename := fmt.Sprintf("deploy-%s.sh", strings.ToLower(m.provider))
		if err := os.WriteFile(filename, []byte(script), 0755); err != nil {
			return &rotationErrMsg{err}
		}

		return &scriptGeneratedMsg{filename: filename}
	}
}

type scriptGeneratedMsg struct{ filename string }
type rotationMsg struct{}
type statusMsg struct{ lastRotated time.Time }
type rotationErrMsg struct{ err error }

func (e *rotationErrMsg) Error() string {
	return e.err.Error()
}

type rotationStartedMsg struct{}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
