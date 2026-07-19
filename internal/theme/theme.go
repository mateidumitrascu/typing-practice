// Package theme defines the built-in themes as color tokens.
// Both clients render from these; adding a theme means adding one entry here.
package theme

type Colors struct {
	Bg      string `json:"bg"`
	Surface string `json:"surface"`
	Text    string `json:"text"`
	Subtext string `json:"subtext"`
	Accent  string `json:"accent"`
	Error   string `json:"error"`
	Success string `json:"success"`
}

type Theme struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Dark   bool   `json:"dark"`
	Colors Colors `json:"colors"`
}

var themes = []Theme{
	{
		Name: "carbon", Label: "Carbon", Dark: true,
		Colors: Colors{
			Bg: "#0f1011", Surface: "#191a1c", Text: "#dcdcdd", Subtext: "#63666e",
			Accent: "#f5c451", Error: "#e5484d", Success: "#46a758",
		},
	},
	{
		Name: "paper", Label: "Paper", Dark: false,
		Colors: Colors{
			Bg: "#faf9f7", Surface: "#eeece7", Text: "#2a2a28", Subtext: "#8a877f",
			Accent: "#3b5bdb", Error: "#c92a2a", Success: "#2f9e44",
		},
	},
	{
		Name: "ember", Label: "Ember", Dark: true,
		Colors: Colors{
			Bg: "#1c1614", Surface: "#28201c", Text: "#e8ddd4", Subtext: "#8a7568",
			Accent: "#e8846b", Error: "#ff6b6b", Success: "#a3be8c",
		},
	},
	{
		Name: "fjord", Label: "Fjord", Dark: true,
		Colors: Colors{
			Bg: "#10141c", Surface: "#1a2030", Text: "#d8dee9", Subtext: "#616e88",
			Accent: "#88c0d0", Error: "#bf616a", Success: "#a3be8c",
		},
	},
}

func All() []Theme {
	return themes
}

func Get(name string) (Theme, bool) {
	for _, t := range themes {
		if t.Name == name {
			return t, true
		}
	}
	return Theme{}, false
}

func Default() Theme {
	return themes[0]
}
