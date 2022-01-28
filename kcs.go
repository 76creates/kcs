package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/util/homedir"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type model struct {
	selected string
	contextChoiceList []string
	contextChoiceListFiltered []string
	contextChoiceListVisible []string
	cursor int
	cursorTop int
	findString string
	configPath string
	kube *kubeConfig
}

type kubeConfig struct {
	ApiVersion interface{} `yaml:"apiVersion"`
	Clusters []cluster `yaml:"clusters"`
	Contexts interface{} `yaml:"contexts"`
	CurrentContext string `yaml:"current-context"`
	Kind interface{} `yaml:"kind"`
	Preferences interface{} `yaml:"preferences"`
	Users interface{} `yaml:"users"`
}

type clusterData struct {
	CAD *string `yaml:"certificate-authority-data,omitempty"`
	Server string `yaml:"server"`
}

type cluster struct {
	ClusterData clusterData `yaml:"cluster"`
	Name string `yaml:"name"`
}

var (
	primary = lipgloss.AdaptiveColor{ Light: "#7158e2", Dark: "#FFC312" }
	secondary = lipgloss.AdaptiveColor{ Light: "#EE5A24", Dark: "#ED4C67" }
	forPrimary = lipgloss.AdaptiveColor{ Light: "#ffffff", Dark: "#000000" }
	forSecondary = lipgloss.AdaptiveColor{ Light: "#ffffff", Dark: "#000000" }
	forDimmed = lipgloss.AdaptiveColor{ Light: "#99A4A8", Dark: "#3D474A" }

	selected = lipgloss.NewStyle().
		Foreground(primary).
		Bold(true)
	unselected = lipgloss.NewStyle()
	cursor = lipgloss.NewStyle().
		Background(primary).
		Foreground(forPrimary).
		Bold(true)
	cursorSelected = cursor.Copy().
		Underline(true)
	search = lipgloss.NewStyle().
		Background(secondary).
		Foreground(forSecondary).
		Bold(true)
	searchEmpty = lipgloss.NewStyle().
		Foreground(forDimmed).
		Underline(true).
		Italic(true)
	title = lipgloss.NewStyle().
		Bold(true)

	mainArea = lipgloss.NewStyle().MaxHeight(6)

	done = false
)


func main() {
	config := filepath.Join(homedir.HomeDir(), ".kube", "config")
	f, err := os.Open(config)
	if err != nil {
		log.Fatal(err.Error())
	}

	kc := new(kubeConfig)
	err =  yaml.NewDecoder(f).Decode(kc)
	if err != nil {
		log.Fatal(err.Error())
	}

	var clusters []string
	for _, cluster := range kc.Clusters {
		clusters = append(clusters, cluster.Name)
	}

	m := model{
		cursor: 0,
		cursorTop: 0,
		contextChoiceList: clusters,
		contextChoiceListFiltered: clusters,
		selected: kc.CurrentContext,
		configPath: config,
		kube: kc,
	}
	m.setVisible()
	p := tea.NewProgram(&m)
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func (m *model)Init() tea.Cmd {


	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter", " ":
			m.selected = m.contextChoiceListVisible[m.cursor - m.cursorTop]
			m.kube.CurrentContext = m.selected
			done = true
			m.setContext()
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.contextChoiceList)-1 {
				m.cursor++
			}
		case "backspace":
			if len(m.findString) == 1 {
				m.findString = ""
			} else if len(m.findString) > 0 {
				m.findString = m.findString[:len(m.findString)-1]
			}
		default:
			if len(msg.String()) == 1 {
				r := msg.Runes[0]
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					m.findString += string(r)
				}
			}
		}
	}
	m.filterList()
	m.normalizeCursor()
	m.setVisible()

	return m, nil
}

func (m *model)  View() string {
	if done {
		return cursor.Render(" " + m.selected + " ") + "\n"
	}
	var s, f string
	f = "🔎 "
	if len(m.findString) > 0 {
		f += search.Render(" " +m.findString + " ")
	} else {
		f += searchEmpty.Render("type to search")
	}
	s = fmt.Sprintf("%s %s\n", title.Render("Select context:"), f)
	for i, cluster := range m.contextChoiceListVisible {
		_cluster := unselected.Render(" " + cluster + " ")
		if cluster == m.selected {
			_cluster = selected.Render(" " + cluster + " ")
		}
		if  m.cursor - m.cursorTop == i {
			if cluster == m.selected {
				_cluster = cursorSelected.Render(" " + cluster + " ")
			} else {
				_cluster = cursor.Render(" " + cluster + " ")
			}
		}
		s += fmt.Sprintf("%s\n", _cluster)
	}

	return mainArea.Render(s)
}

func (m *model)setVisible() {
	if len(m.contextChoiceListFiltered) <= mainArea.GetMaxHeight()-1 {
		m.contextChoiceListVisible = m.contextChoiceListFiltered
		return
	}
	m.contextChoiceListVisible = m.contextChoiceListFiltered[m.cursorTop:m.cursorTop+mainArea.GetMaxHeight()-1]
}

func (m *model)normalizeCursor() {
	if len(m.contextChoiceListFiltered) == 0 {
		m.cursor = 0
	} else if m.cursor > len(m.contextChoiceListFiltered) - 1 {
		m.cursor = len(m.contextChoiceListFiltered) - 1
	}

	if m.cursor >= m.cursorTop && m.cursor < m.cursorTop + (mainArea.GetMaxHeight() -1){
		// if last item but max is not showing most it can, happens when filtering
		if m.cursor == (len(m.contextChoiceListFiltered) - 1) {
			// in [2][1] situations where there is only one more element remaining
			if len(m.contextChoiceListFiltered) < (mainArea.GetMaxHeight() -1) {
				m.cursorTop = 0
			} else {
				m.cursorTop = m.cursor - (mainArea.GetMaxHeight() -2)
			}
		}
	} else if m.cursor < m.cursorTop {
		if m.cursor == (len(m.contextChoiceListFiltered) - 1) {
			// in [2][1] situations where there is only one more element remaining
			if len(m.contextChoiceListFiltered) < (mainArea.GetMaxHeight() -1) {
				m.cursorTop = 0
			} else {
				m.cursorTop = m.cursor - (mainArea.GetMaxHeight() -2)
			}
		} else {
			m.cursorTop = m.cursor
		}
	} else {
		m.cursorTop = m.cursor - (mainArea.GetMaxHeight()-2)
	}
}
func (m *model)filterList() {
	if len(m.findString) > 0 {
		if len(m.findString) == 0 {
			m.contextChoiceListFiltered = m.contextChoiceList
			return
		}
		m.contextChoiceListFiltered = []string{}
		for _, ch := range m.contextChoiceList {
			if strings.Contains(ch, m.findString) {
				m.contextChoiceListFiltered = append(m.contextChoiceListFiltered, ch)
			}
		}
	} else {
		m.contextChoiceListFiltered = m.contextChoiceList
	}
}

func (m *model)setContext() {
	f, err := os.OpenFile(m.configPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC ,0644)
	if err != nil {
		log.Fatal(err.Error())
	}

	err = yaml.NewEncoder(f).Encode(m.kube)
	if err != nil {
		log.Fatal(err.Error())
	}
}