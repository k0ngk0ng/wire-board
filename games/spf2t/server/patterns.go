package spf2t

// characterDef is the server-owned character roster. The 12x6 pattern is the
// original game's incoming-counter-gem layout, read row-major into a 72-item
// stream. The frontend receives names and patterns through View so the roster
// never has to be duplicated in TypeScript.
type characterDef struct {
	ID      string
	Name    string
	Pattern []string
}

var characterDefs = []characterDef{
	{
		ID:   "morrigan",
		Name: "莫莉卡",
		Pattern: []string{
			"ybggby", "ybggby", "byrryb", "byrryb",
			"gryyrg", "gryyrg", "rgbbgr", "rgbbgr",
			"ybggby", "ybggby", "byrryb", "byrryb",
		},
	},
	{
		ID:   "chunli",
		Name: "春丽",
		Pattern: []string{
			"yyrrgg", "yyrrgg", "rrggbb", "rrggbb",
			"ggbbyy", "ggbbyy", "bbyyrr", "bbyyrr",
			"yyrrgg", "yyrrgg", "rrggbb", "rrggbb",
		},
	},
	{
		ID:   "ryu",
		Name: "隆",
		Pattern: []string{
			"rgbyrg", "rgbyrg", "rgbyrg", "rgbyrg",
			"rgbyrg", "rgbyrg", "rgbyrg", "rgbyrg",
			"rgbyrg", "rgbyrg", "rgbyrg", "rgbyrg",
		},
	},
	{
		ID:   "ken",
		Name: "肯",
		Pattern: []string{
			"yyyyyy", "bbbbbb", "gggggg", "rrrrrr",
			"yyyyyy", "bbbbbb", "gggggg", "rrrrrr",
			"yyyyyy", "bbbbbb", "gggggg", "rrrrrr",
		},
	},
	{
		ID:   "hsienko",
		Name: "泪泪",
		Pattern: []string{
			"ggrryy", "bggrry", "bbggrr", "rbbggr",
			"yybbgg", "ryybbg", "rryybb", "grryyb",
			"ggrryy", "bggrry", "bbggrr", "ybbggr",
		},
	},
	{
		ID:   "donovan",
		Name: "多诺万",
		Pattern: []string{
			"gggbbb", "gggbbb", "ryryry", "ryryry",
			"gggbbb", "gggbbb", "ryryry", "ryryry",
			"gggbbb", "gggbbb", "ryryry", "ryryry",
		},
	},
	{
		ID:   "felicia",
		Name: "菲莉西亚",
		Pattern: []string{
			"gbbrry", "gbbrry", "grrbby", "grrbby",
			"gbbrry", "gbbrry", "grrbby", "grrbby",
			"gbbrry", "gbbrry", "grrbby", "grrbby",
		},
	},
	{
		ID:   "sakura",
		Name: "樱",
		Pattern: []string{
			"gbbbby", "grrrry", "gbbbby", "grrrry",
			"gbbbby", "grrrry", "gbbbby", "grrrry",
			"gbbbby", "grrrry", "gbbbby", "grrrry",
		},
	},
}

var characterIndex = func() map[string]characterDef {
	m := make(map[string]characterDef, len(characterDefs))
	for _, c := range characterDefs {
		m[c.ID] = c
	}
	return m
}()

func validCharacter(id string) bool {
	_, ok := characterIndex[id]
	return ok
}

func characterDefOf(id string) (characterDef, bool) {
	c, ok := characterIndex[id]
	return c, ok
}

func characterViews() []CharacterView {
	out := make([]CharacterView, 0, len(characterDefs))
	for _, c := range characterDefs {
		out = append(out, CharacterView{ID: c.ID, Name: c.Name, Pattern: append([]string{}, c.Pattern...)})
	}
	return out
}

func patternAt(character string, index int) (color string, column int) {
	c, ok := characterDefOf(character)
	if !ok {
		return "", 0
	}
	index %= 72
	if index < 0 {
		index += 72
	}
	row := index / 6
	col := index % 6
	ch := c.Pattern[row][col]
	switch ch {
	case 'r':
		return ColorRed, col
	case 'y':
		return ColorYellow, col
	case 'g':
		return ColorGreen, col
	case 'b':
		return ColorBlue, col
	}
	return "", col
}

func patternFor(character string) []string {
	c, ok := characterDefOf(character)
	if !ok {
		return nil
	}
	return append([]string{}, c.Pattern...)
}
