package bot

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/igorvoltaic/keenetic-bot/router"
	tele "gopkg.in/telebot.v3"
)

func (b *Bot) handleStart(c tele.Context) error {
	return c.Send("Keenetic bot.\n\n" +
		"/sysinfo — состояние роутера\n" +
		"/clients — подключённые устройства\n" +
		"/xkeen <start|stop|restart|status> — управление xkeen\n" +
		"/reboot — перезагрузить роутер")
}

func (b *Bot) handleSysInfo(c tele.Context) error {
	info, err := router.SystemInfo()
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %v", err))
	}

	d := info.Uptime
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	memUsed := info.MemTotal - info.MemAvail
	memPct := 0
	if info.MemTotal > 0 {
		memPct = int(memUsed * 100 / info.MemTotal)
	}

	lines := []string{
		fmt.Sprintf("Аптайм: %dд %dч %dм", days, hours, mins),
		fmt.Sprintf("Нагрузка: %s / %s / %s", info.Load1, info.Load5, info.Load15),
		fmt.Sprintf("RAM: %s / %s (%d%%)", formatBytes(memUsed), formatBytes(info.MemTotal), memPct),
	}
	if xrayUp, err := router.ProcessUptime("xray"); err == nil {
		xd := int(xrayUp.Hours()) / 24
		xh := int(xrayUp.Hours()) % 24
		xm := int(xrayUp.Minutes()) % 60
		lines = append(lines, fmt.Sprintf("xray: %dд %dч %dм", xd, xh, xm))
	}
	if xkeenStat, err := os.Stat(b.cfg.Router.XkeenPath); err == nil {
		lines = append(lines, fmt.Sprintf("XKeen: %s", xkeenStat.ModTime().Format("02 Jan 2006")))
	}
	if geoTime, err := router.GeoUpdateTime(b.cfg.Router.XkeenDatDir); err == nil {
		lines = append(lines, fmt.Sprintf("Geo: %s", geoTime.Format("02 Jan 2006 15:04")))
	}
	return c.Send(strings.Join(lines, "\n"))
}

func (b *Bot) handleReboot(c tele.Context) error {
	if err := c.Send("Перезагружаю роутер..."); err != nil {
		return err
	}
	if err := router.Reboot(); err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %v", err))
	}
	return nil
}

func (b *Bot) handleXkeen(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("Использование: /xkeen <start|stop|restart|status>")
	}
	action := args[0]
	switch action {
	case "start", "stop", "restart", "status":
	default:
		return c.Send("Допустимые действия: start, stop, restart, status")
	}

	if err := c.Send(fmt.Sprintf("xkeen %s...", action)); err != nil {
		return err
	}
	out, err := router.XkeenCmd(b.cfg.Router.XkeenPath, action)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %v\n%s", err, out))
	}
	if action == "status" {
		if xrayUp, err := router.ProcessUptime("xray"); err == nil {
			xd := int(xrayUp.Hours()) / 24
			xh := int(xrayUp.Hours()) % 24
			xm := int(xrayUp.Minutes()) % 60
			out += fmt.Sprintf("\nxray uptime: %dд %dч %dм", xd, xh, xm)
		}
	}
	if out == "" {
		return c.Send("Готово.")
	}
	return c.Send(out)
}

func (b *Bot) handleClients(c tele.Context) error {
	clients, err := router.ConnectedClients()
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %v", err))
	}
	if len(clients) == 0 {
		return c.Send("Нет подключённых устройств.")
	}

	sort.Slice(clients, func(i, j int) bool {
		ni, nj := clients[i].Network, clients[j].Network
		if ni == nj {
			return clients[i].Name < clients[j].Name
		}
		if ni == "Home" {
			return true
		}
		if nj == "Home" {
			return false
		}
		return ni < nj
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Подключено: %d\n\n", len(clients)))
	for _, cl := range clients {
		name := cl.Name
		if name == "" {
			name = cl.MAC
		}
		network := cl.Network
		if network == "" {
			network = "—"
		}
		traffic := ""
		if cl.RxBytes > 0 || cl.TxBytes > 0 {
			traffic = fmt.Sprintf(" ↓%s ↑%s", formatBytes(cl.RxBytes), formatBytes(cl.TxBytes))
		}
		sb.WriteString(fmt.Sprintf("• %s — %s [%s]%s\n", name, cl.IP, network, traffic))
	}
	return c.Send(sb.String())
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1fGB", float64(b)/1024/1024/1024)
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/1024/1024)
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}
