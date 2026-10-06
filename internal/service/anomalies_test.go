package service

import (
	"slices"
	"testing"

	"partnerops/internal/partner"
)

func row(edoID, inn, owner, login string, packets int64, limit *int64) partner.EDOBillingRow {
	return partner.EDOBillingRow{
		EDOID: edoID, INN: inn, KPP: "770001001", ClientName: "ООО Тест",
		Owner: owner, Login: login, Packets: packets, Limit: limit,
	}
}

//go:fix inline
func limitOf(v int64) *int64 { return new(v) }

func TestOrphanWithTrafficIsHighConfidence(t *testing.T) {
	// Документы идут, а тарифа, к которому их отнести, нет — прямые деньги.
	rows := []partner.EDOBillingRow{
		row("2AE-A", "7700000001", "", "user", 56, nil),
	}

	found := FindAnomalies(rows, nil)

	if len(found) != 1 {
		t.Fatalf("находок %d, ожидали 1", len(found))
	}
	if found[0].Kind != KindOrphanWithTraffic {
		t.Errorf("kind = %q, ожидали %q", found[0].Kind, KindOrphanWithTraffic)
	}
	if found[0].Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, ожидали %q", found[0].Confidence, ConfidenceHigh)
	}
}

func TestOrphanWithoutTrafficIsLowConfidence(t *testing.T) {
	rows := []partner.EDOBillingRow{
		row("2AE-B", "7700000002", "", "user", 0, nil),
	}

	found := FindAnomalies(rows, nil)

	if len(found) != 1 {
		t.Fatalf("находок %d, ожидали 1", len(found))
	}
	if found[0].Kind != KindOrphanIdle {
		t.Errorf("kind = %q, ожидали %q", found[0].Kind, KindOrphanIdle)
	}
	if found[0].Confidence != ConfidenceLow {
		t.Errorf("confidence = %q, ожидали low", found[0].Confidence)
	}
}

func TestOwnerLostBetweenSnapshots(t *testing.T) {
	previous := map[string]string{"2AE-C": "CL-1 - Иванов"}
	rows := []partner.EDOBillingRow{
		row("2AE-C", "7700000003", "", "user", 0, nil),
	}

	found := FindAnomalies(rows, previous)

	var kinds []string
	for _, a := range found {
		kinds = append(kinds, a.Kind)
	}
	if !contains(kinds, KindOwnerLost) {
		t.Errorf("виды находок %v, ожидали среди них %q", kinds, KindOwnerLost)
	}
	for _, a := range found {
		if a.Kind == KindOwnerLost && a.Confidence != ConfidenceHigh {
			t.Errorf("потеря владельца должна быть высокой уверенности, получили %q", a.Confidence)
		}
	}
}

func TestHealthyRowsProduceNothing(t *testing.T) {
	rows := []partner.EDOBillingRow{
		row("2AE-D", "7700000004", "CL-1 - Иванов", "user", 10, limitOf(100)),
		row("2AE-E", "7700000005", "FR-FR-2 - Петров", "other", 0, limitOf(50)),
	}

	if found := FindAnomalies(rows, nil); len(found) != 0 {
		t.Errorf("находок %d, ожидали 0: здоровые строки сигналов не дают: %+v", len(found), found)
	}
}

func TestDuplicateWithOwnersIsNotAnomaly(t *testing.T) {
	// Один ИНН с двумя полноценными идентификаторами — законная ситуация:
	// у клиента разные виды деятельности. Сигналом это быть не должно.
	rows := []partner.EDOBillingRow{
		row("2AE-F", "7700000006", "CL-1 - Иванов", "user", 5, limitOf(50)),
		row("2AE-G", "7700000006", "CL-2 - Иванов", "user", 7, limitOf(50)),
	}

	if found := FindAnomalies(rows, nil); len(found) != 0 {
		t.Errorf("находок %d, ожидали 0: дубль с полноценными владельцами законен: %+v",
			len(found), found)
	}
}

func TestFingerprintChangesWithState(t *testing.T) {
	first := FindAnomalies([]partner.EDOBillingRow{
		row("2AE-H", "7700000007", "", "user", 10, nil),
	}, nil)
	second := FindAnomalies([]partner.EDOBillingRow{
		row("2AE-H", "7700000007", "", "user", 20, nil),
	}, nil)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("ожидали по одной находке, получили %d и %d", len(first), len(second))
	}
	if first[0].StateFingerprint == second[0].StateFingerprint {
		t.Error("отпечаток обязан меняться вместе с состоянием, иначе подтверждение скроет новую поломку")
	}
}

func contains(list []string, value string) bool {
	return slices.Contains(list, value)
}
