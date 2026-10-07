package delivery

type Channel string

const (
	ChannelSystem    Channel = "system"
	ChannelEphemeral Channel = "ephemeral"
	ChannelDeny      Channel = "deny"
)

type Moment string

const (
	MomentTaskStart   Moment = "task_start"
	MomentFirstEntry  Moment = "first_entry"
	MomentFirstTouch  Moment = "first_touch"
	MomentAboutToEdit Moment = "about_to_edit"
	MomentError       Moment = "error"
	MomentGate        Moment = "gate"
)

func DefaultBudgets() map[Moment]int {
	return map[Moment]int{
		MomentTaskStart:   500,
		MomentFirstEntry:  150,
		MomentFirstTouch:  150,
		MomentAboutToEdit: 50,
		MomentError:       200,
		MomentGate:        200,
	}
}
