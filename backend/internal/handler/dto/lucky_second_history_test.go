package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLuckySecondHistoryPublicCampaignName(t *testing.T) {
	r := &service.RedeemCode{Type: service.RedeemTypeLuckySecondReward, Notes: "国庆幸运秒", Value: 0.000001}
	public := RedeemCodeFromService(r)
	require.Equal(t, r.Value, public.Value)
	require.NotNil(t, public.Notes)
	require.Equal(t, r.Notes, *public.Notes)
	require.Equal(t, r.Notes, RedeemCodeFromServiceAdmin(r).Notes)
	r.Type = service.RedeemTypeBalance
	require.Nil(t, RedeemCodeFromService(r).Notes, "ordinary internal notes stay private")
}
