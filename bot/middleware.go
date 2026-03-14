package bot

import (
	tele "gopkg.in/telebot.v3"
)

// allowOnly returns a middleware that rejects requests from users not in the allowed list.
func allowOnly(ids []int64) tele.MiddlewareFunc {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if _, ok := set[c.Sender().ID]; !ok {
				return c.Send("Not authorized.")
			}
			return next(c)
		}
	}
}
