package zone

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
)

// ParamKind is how the server parses a known parameter's value, which
// decides the values a zone may carry for it.
type ParamKind int

const (
	// BoolParam is "true" or "false" (MultiValueSet.getBool reads any other
	// text as false, silently).
	BoolParam ParamKind = iota
	// IntParam is a 32-bit integer (Integer.parseInt).
	IntParam
	// LongParam is a 64-bit integer (Long.parseLong).
	LongParam
	// DoubleParam is a decimal number (Double.parseDouble).
	DoubleParam
	// ChoiceParam is one of ParamSpec.Choices, in its exact case (an enum's
	// valueOf).
	ChoiceParam
	// MessageParam is a SystemMsg id, or -1 for none (SystemMsg.valueOf
	// throws on an unknown id).
	MessageParam
	// SkillParam is a skill as "id level", separated by spaces, commas or
	// semicolons.
	SkillParam
	// ActionsParam is a list of ParamSpec.Choices (the action names
	// Player.isActionBlocked is asked about), separated by spaces, commas or
	// semicolons.
	ActionsParam
	// TextParam is any text.
	TextParam
)

// ParamSpec is a parameter the server's ZoneTemplate reads.
type ParamSpec struct {
	Name string
	Kind ParamKind
	// Default is what the server uses when the zone does not set the
	// parameter; "" for a text, skill or action list means none.
	Default string
	// Choices are the values of a BoolParam or ChoiceParam and the names
	// an ActionsParam lists.
	Choices []string
}

var (
	boolChoices = []string{"true", "false"}
	// blockedActions are every name the server's code passes to
	// Player.isActionBlocked.
	blockedActions = []string{
		"drop_item", "drop_merchant_guard", "exp_lost", "open_private_store",
		"open_private_workshop", "pvp_point_increase", "ride", "save_bookmark", "use_bookmark",
	}
)

// KnownParams are the parameters the ZoneTemplate constructor reads
// (majestic-java ZoneTemplate.java), with the server's defaults: enabled
// and default first, then the constructor's order. Any other name is a
// free parameter, kept for the scripts that read it.
var KnownParams = []ParamSpec{
	{Name: "enabled", Kind: BoolParam, Default: "true", Choices: boolChoices},
	{Name: "default", Kind: BoolParam, Default: "true", Choices: boolChoices},
	{Name: "entering_message_no", Kind: MessageParam, Default: "-1"},
	{Name: "leaving_message_no", Kind: MessageParam, Default: "-1"},
	{Name: "entering_custom_message", Kind: TextParam},
	{Name: "leaving_custom_message", Kind: TextParam},
	{Name: "target", Kind: ChoiceParam, Default: "pc", Choices: []string{"pc", "npc", "only_pc"}},
	{Name: "affect_race", Kind: ChoiceParam, Default: "all", Choices: []string{"all", "human", "elf", "darkelf", "orc", "dwarf"}},
	{Name: "skill_name", Kind: SkillParam},
	{Name: "skill_prob", Kind: IntParam, Default: "100"},
	{Name: "initial_delay", Kind: IntParam, Default: "1"},
	{Name: "unit_tick", Kind: IntParam, Default: "1"},
	{Name: "random_time", Kind: IntParam, Default: "0"},
	{Name: "move_bonus", Kind: DoubleParam, Default: "0.0"},
	{Name: "hp_regen_bonus", Kind: DoubleParam, Default: "0.0"},
	{Name: "mp_regen_bonus", Kind: DoubleParam, Default: "0.0"},
	{Name: "damage_on_hp", Kind: IntParam, Default: "0"},
	{Name: "damage_on_mp", Kind: IntParam, Default: "0"},
	{Name: "message_no", Kind: MessageParam, Default: "-1"},
	{Name: "eventId", Kind: IntParam, Default: "0"},
	{Name: "restart_time", Kind: LongParam, Default: "0"},
	{Name: "restart_allowed_time", Kind: TextParam},
	{Name: "blocked_actions", Kind: ActionsParam, Choices: blockedActions},
	{Name: "index", Kind: IntParam, Default: "0"},
	{Name: "taxById", Kind: IntParam, Default: "0"},
	{Name: "jumping_track", Kind: IntParam, Default: "-1"},
}

// reservedParams are the names the ZoneParser stores its own values under
// in the same set as the <set> parameters (zone name and type, shapes,
// restart points): a <set> with one of them breaks the zone.
var reservedParams = []string{"name", "type", "territory", "restart_points", "PKrestart_points"}

// KnownParam returns the spec of the known parameter called name.
func KnownParam(name string) (ParamSpec, bool) {
	i := slices.IndexFunc(KnownParams, func(s ParamSpec) bool { return s.Name == name })
	if i < 0 {
		return ParamSpec{}, false
	}
	return KnownParams[i], true
}

var (
	// listSep is the separator the server splits skill and list values on.
	listSep = regexp.MustCompile(`[\s,;]+`)
	decimal = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)
)

// Check reports whether v is a value of s's kind the server parses.
func (s ParamSpec) Check(v string) error {
	bad := func(want string) error {
		return fmt.Errorf("zone: %s takes %s, not %q", s.Name, want, v)
	}
	switch s.Kind {
	case BoolParam, ChoiceParam:
		if !slices.Contains(s.Choices, v) {
			return bad(fmt.Sprint(s.Choices))
		}
	case IntParam:
		if _, err := strconv.ParseInt(v, 10, 32); err != nil {
			return bad("a 32-bit integer")
		}
	case LongParam:
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			return bad("an integer")
		}
	case DoubleParam:
		if !decimal.MatchString(v) {
			return bad("a decimal number")
		}
	case MessageParam:
		id, err := strconv.ParseInt(v, 10, 32)
		if err != nil || id != -1 && !systemMessage(int(id)) {
			return bad("a SystemMsg id or -1")
		}
	case SkillParam:
		f := listSep.Split(v, -1)
		if len(f) != 2 {
			return bad(`"id level"`)
		}
		for _, n := range f {
			if _, err := strconv.ParseInt(n, 10, 32); err != nil {
				return bad(`"id level"`)
			}
		}
	case ActionsParam:
		for _, a := range listSep.Split(v, -1) {
			if !slices.Contains(s.Choices, a) {
				return bad(fmt.Sprintf("action names out of %v", s.Choices))
			}
		}
	}
	return nil
}

// systemMessage reports whether id is a SystemMsg id.
func systemMessage(id int) bool {
	_, found := slices.BinarySearchFunc(systemMsgIDs[:], id, func(r [2]int, id int) int {
		switch {
		case r[1] < id:
			return -1
		case r[0] > id:
			return 1
		}
		return 0
	})
	return found
}
