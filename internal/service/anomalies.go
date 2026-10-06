package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"partnerops/internal/partner"
	"partnerops/internal/store"
)

// Виды находок.
const (
	// KindOrphanWithTraffic — трафик идёт, а тарифа, к которому его отнести, нет.
	KindOrphanWithTraffic = "orphan_with_traffic"
	// KindOwnerLost — владелец был в прошлом снимке и пропал.
	KindOwnerLost = "owner_lost"
	// KindOrphanIdle — идентификатор без владельца и без трафика.
	KindOrphanIdle = "orphan_idle"
	// KindDisappeared — идентификатор пропал из отчёта: один снимок без него —
	// подозрение, два подряд — подтверждение (спека §4.2, сигнал 3).
	KindDisappeared = "disappeared"
	// KindReplaced — у той же организации или логина старый идентификатор
	// пропал, а новый появился: одно событие вместо двух тревог (сигнал 4).
	KindReplaced = "replaced"
)

// Уровни уверенности.
const (
	ConfidenceHigh = "high"
	ConfidenceLow  = "low"
)

// Anomaly — найденная аномалия до записи в базу.
type Anomaly struct {
	EDOID            string
	Kind             string
	Confidence       string
	StateFingerprint string
	INN              string
	KPP              string
	ClientName       string
	Login            string
	Details          string
}

// FindAnomalies ищет поломки в строках снапшота.
//
// previousOwners — владельцы из предыдущего снимка, ключ это идентификатор.
// Может быть nil: тогда переходы состояний не проверяются.
//
// Несколько идентификаторов на одну организацию сигналом НЕ считаются:
// проверено на живых данных, что это бывает законно (разные виды деятельности,
// один логин с двумя ИНН). Отличить законный дубль от поломки структурно нельзя.
func FindAnomalies(rows []partner.EDOBillingRow, previousOwners map[string]string) []Anomaly {
	var found []Anomaly

	for _, r := range rows {
		if r.EDOID == "" {
			continue
		}

		hadOwner := false
		if previousOwners != nil {
			previous, seen := previousOwners[r.EDOID]
			hadOwner = seen && previous != ""
		}

		switch {
		case r.Owner == "" && r.Packets > 0:
			found = append(found, Anomaly{
				EDOID: r.EDOID, Kind: KindOrphanWithTraffic, Confidence: ConfidenceHigh,
				StateFingerprint: fingerprint(r.EDOID, r.Owner, r.Login, r.Packets),
				INN:              r.INN, KPP: r.KPP, ClientName: r.ClientName, Login: r.Login,
				Details: fmt.Sprintf(
					"Трафик %d пакетов не привязан ни к какому тарифу: лимита и владельца нет.",
					r.Packets),
			})

		case r.Owner == "":
			found = append(found, Anomaly{
				EDOID: r.EDOID, Kind: KindOrphanIdle, Confidence: ConfidenceLow,
				StateFingerprint: fingerprint(r.EDOID, r.Owner, r.Login, r.Packets),
				INN:              r.INN, KPP: r.KPP, ClientName: r.ClientName, Login: r.Login,
				Details: "Идентификатор без владельца и без трафика.",
			})
		}

		if hadOwner && r.Owner == "" {
			found = append(found, Anomaly{
				EDOID: r.EDOID, Kind: KindOwnerLost, Confidence: ConfidenceHigh,
				StateFingerprint: fingerprint(r.EDOID, previousOwners[r.EDOID], r.Login, r.Packets),
				INN:              r.INN, KPP: r.KPP, ClientName: r.ClientName, Login: r.Login,
				Details: fmt.Sprintf("Владелец был «%s», теперь поле пусто.",
					previousOwners[r.EDOID]),
			})
		}
	}

	return found
}

// fingerprint описывает состояние, породившее находку. Подтверждение «это законно»
// гасит только его: изменилось состояние — сигнал поднимется снова.
func fingerprint(parts ...any) string {
	sum := sha256.Sum256([]byte(fmt.Sprint(parts...)))
	return hex.EncodeToString(sum[:16])
}

// ownerPhrase — владелец для текста находки: пустое поле отчёта не выводим как «».
func ownerPhrase(owner string) string {
	if strings.TrimSpace(owner) == "" {
		return "владельца не было"
	}
	return "владелец «" + owner + "»"
}

// FindVanished ищет идентификаторы реестра, которых нет в новом снимке.
//
// known — реестр до обновления этим снимком. Каждый обработанный снимок
// сдвигает LastSeenAt всех своих строк, поэтому самый поздний LastSeenAt —
// время предыдущего снимка. Кого не было и в нём, тот пропал два снимка
// подряд: подтверждение. Кто был в предыдущем, но нет в этом, — подозрение:
// один сбой на стороне 1С не должен объявлять идентификатор пропавшим.
// Отпечаток у обоих один (идентификатор и когда его видели последним), так что
// запись подозрения повышается до подтверждения, а не дублируется.
//
// Если у той же организации (ИНН) или того же логина в снимке есть
// идентификатор, которого не было в реестре или который появился позже
// пропавшего, это замена: одно событие replaced по новому идентификатору.
func FindVanished(rows []partner.EDOBillingRow, known []store.IdentifierRecord, period string) []Anomaly {
	present := map[string]partner.EDOBillingRow{}
	for _, r := range rows {
		if r.EDOID != "" {
			present[r.EDOID] = r
		}
	}
	var previousAt time.Time
	firstSeen := map[string]time.Time{}
	for _, k := range known {
		if k.LastSeenAt.After(previousAt) {
			previousAt = k.LastSeenAt
		}
		firstSeen[k.EDOID] = k.FirstSeenAt
	}

	var found []Anomaly
	for _, old := range known {
		if _, ok := present[old.EDOID]; ok {
			continue
		}
		if successor, ok := replacement(old, rows, firstSeen); ok {
			found = append(found, Anomaly{
				EDOID: successor.EDOID, Kind: KindReplaced, Confidence: ConfidenceHigh,
				StateFingerprint: fingerprint(old.EDOID, successor.EDOID),
				INN:              successor.INN, KPP: successor.KPP, ClientName: successor.ClientName,
				Login: successor.Login,
				Details: fmt.Sprintf("Вместо %s (последний раз %s, период %s) в отчёте за %s появился этот идентификатор.",
					old.EDOID, old.LastSeenAt.Format("02.01.2006"), old.LastPeriod, period),
			})
			continue
		}
		anomaly := Anomaly{
			EDOID: old.EDOID, Kind: KindDisappeared, Confidence: ConfidenceHigh,
			StateFingerprint: fingerprint(old.EDOID, old.LastSeenAt.Unix()),
			INN:              old.INN, KPP: old.KPP, ClientName: old.ClientName, Login: old.Login,
			Details: fmt.Sprintf("Нет в двух снимках подряд; последний раз был %s (период %s), %s.",
				old.LastSeenAt.Format("02.01.2006"), old.LastPeriod, ownerPhrase(old.OwnerRaw)),
		}
		if !old.LastSeenAt.Before(previousAt) {
			anomaly.Confidence = ConfidenceLow
			anomaly.Details = fmt.Sprintf("Подозрение: нет в отчёте за %s, был в прошлом снимке (период %s). "+
				"Подтвердится, если не появится и в следующем.", period, old.LastPeriod)
		}
		found = append(found, anomaly)
	}
	return found
}

// replacement ищет в снимке преемника пропавшего идентификатора: та же
// организация или тот же логин, а сам он новый — нет в реестре или появился
// после того, как пропавший видели последним.
func replacement(
	old store.IdentifierRecord, rows []partner.EDOBillingRow, firstSeen map[string]time.Time,
) (partner.EDOBillingRow, bool) {
	for _, r := range rows {
		if r.EDOID == "" || r.EDOID == old.EDOID {
			continue
		}
		sameOrg := old.INN != "" && r.INN == old.INN
		sameLogin := old.Login != "" && strings.EqualFold(r.Login, old.Login)
		if !sameOrg && !sameLogin {
			continue
		}
		if seen, known := firstSeen[r.EDOID]; !known || seen.After(old.LastSeenAt) {
			return r, true
		}
	}
	return partner.EDOBillingRow{}, false
}

// TopologyKinds — сигналы, которые гасит реестр ожидаемой топологии. Это
// сигналы о лишнем идентификаторе у организации: запасной без владельца и
// трафика или новый рядом со старым. Трафик мимо тарифа, пропажа владельца
// и пропажа идентификатора — деньги и поломки, их топология не гасит: иначе
// связь «ИНН + идентификатор» без отпечатка скрыла бы будущую поломку.
var TopologyKinds = map[string]bool{KindOrphanIdle: true, KindReplaced: true}

// TopologyKey — ключ связи в реестре топологии.
func TopologyKey(inn, kpp, edoID string) string {
	return strings.TrimSpace(inn) + "/" + strings.TrimSpace(kpp) + "/" + strings.TrimSpace(edoID)
}

// SuppressedByTopology возвращает назначение из реестра топологии, если
// находка этого вида по этой связи там отмечена законной.
func SuppressedByTopology(kind, inn, kpp, edoID string, topology map[string]string) (string, bool) {
	if !TopologyKinds[kind] {
		return "", false
	}
	purpose, ok := topology[TopologyKey(inn, kpp, edoID)]
	return purpose, ok
}
