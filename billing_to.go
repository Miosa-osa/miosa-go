package miosa

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
)

// Wire contract: miosa-compute docs/api/bill-to.md.
//
// The billing account is the organization. Choosing who pays only chooses
// which organization a sandbox or computer create runs in; nothing in billing
// is moved or re-priced.

// BillToSource says where the organization a create bills was chosen.
type BillToSource string

const (
	// BillToSourceRequest: the X-Miosa-Bill-To header or bill_to parameter.
	BillToSourceRequest BillToSource = "request"
	// BillToSourceSetting: the account-wide setting (PUT /bill-to).
	BillToSourceSetting BillToSource = "setting"
	// BillToSourceCredential: the organization the credential is in.
	BillToSourceCredential BillToSource = "credential"
)

// BillToOrganization is an organization a create can bill.
type BillToOrganization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	// Type is "personal" or "company".
	Type string `json:"type"`
	Role string `json:"role"`
}

// BillToOption is one organization the caller may choose.
type BillToOption struct {
	BillToOrganization
	// Viewing is the organization the credential is currently viewing.
	Viewing bool `json:"viewing"`
	// Billing is the organization a create would bill.
	Billing bool `json:"billing"`
	// Pinned is the account-wide setting.
	Pinned bool `json:"pinned"`
}

// BillTo is what a create would bill right now, and the choices.
type BillTo struct {
	Billing BillToOrganization `json:"billing"`
	Source  BillToSource       `json:"source"`
	// SettingPinned is true when an account-wide setting exists.
	SettingPinned bool `json:"setting_pinned"`
	// SettingStale is true when the setting points at an organization the user
	// can no longer use; it is ignored.
	SettingStale bool   `json:"setting_stale"`
	ViewingID    string `json:"viewing_id,omitempty"`
	// CanChange is false for credentials locked to their own organization.
	CanChange bool           `json:"can_change"`
	Options   []BillToOption `json:"options"`
}

// BlockedReason values of Limits.BlockedReasons.
const (
	BlockedInsufficientCredits         = "insufficient_credits"
	BlockedConcurrencyLimitReached     = "concurrency_limit_reached"
	BlockedSpendCapReached             = "spend_cap_reached"
	BlockedMemberUsageCapReached       = "member_usage_cap_reached"
	BlockedMemberConcurrencyCapReached = "member_concurrency_cap_reached"
)

// LimitsCredits is the credit side of Limits (cents; 1 credit = 1 cent).
type LimitsCredits struct {
	BalanceCents   int64 `json:"balance_cents"`
	AvailableCents int64 `json:"available_cents"`
}

// LimitsConcurrency is the running-machine side of Limits. Limit and Remaining
// are nil when the plan has no limit.
type LimitsConcurrency struct {
	Limit     *int `json:"limit"`
	Running   int  `json:"running"`
	Remaining *int `json:"remaining"`
}

// LimitsSpend is the spend-cap side of Limits.
type LimitsSpend struct {
	Mode           string `json:"mode"`
	CapCents       *int64 `json:"cap_cents"`
	AccountedCents int64  `json:"accounted_cents"`
	Status         string `json:"status"`
}

// LimitsMember is the caller's per-member caps in the organization.
type LimitsMember struct {
	UsageCapCents          *int64 `json:"usage_cap_cents"`
	UsageCents             int64  `json:"usage_cents"`
	MaxConcurrentSandboxes *int   `json:"max_concurrent_sandboxes"`
	RunningSandboxes       int    `json:"running_sandboxes"`
}

// Limits are the limits of the organization a create would bill.
type Limits struct {
	Organization BillToOrganization `json:"organization"`
	Source       BillToSource       `json:"source"`
	// CanStart is false when BlockedReasons is non-empty.
	CanStart       bool          `json:"can_start"`
	BlockedReasons []string      `json:"blocked_reasons"`
	Credits        LimitsCredits `json:"credits"`
	Plan           struct {
		Name string `json:"name"`
	} `json:"plan"`
	Concurrency LimitsConcurrency `json:"concurrency"`
	Spend       LimitsSpend       `json:"spend"`
	// Member is nil unless the caller has been given caps in that organization.
	Member *LimitsMember `json:"member"`
}

// BillToService reads and changes who pays for new machines, and reads the
// limits a create would hit. Accessed via Client.BillTo.
//
// A per-request override is a header, not a method: use WithBillTo on the
// client or WithBillToContext on one call's context.
type BillToService struct {
	client *Client
}

// Get returns what a create would bill right now (honoring any override on the
// context or client), and the choices.
func (s *BillToService) Get(ctx context.Context) (*BillTo, error) {
	var out apiResponse[BillTo]
	if err := s.client.getJSON(ctx, "/bill-to", &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Set pins an organization account-wide (id, slug or name). It needs a
// credential that may choose; others get 403 BILL_TO_LOCKED.
func (s *BillToService) Set(ctx context.Context, org string) (*BillTo, error) {
	if org == "" {
		return nil, errors.New("org is required")
	}
	var out apiResponse[BillTo]
	if err := s.client.putJSON(ctx, "/bill-to", map[string]string{"org": org}, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Clear removes the setting: creates follow the credential's organization again.
func (s *BillToService) Clear(ctx context.Context) error {
	return s.client.deleteJSON(ctx, "/bill-to", nil)
}

// Limits returns the limits of the organization a create would bill, with no
// side effects.
func (s *BillToService) Limits(ctx context.Context) (*Limits, error) {
	var out apiResponse[Limits]
	if err := s.client.getJSON(ctx, "/limits", &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// ─── Per-member caps ──────────────────────────────────────────────────────────

// MemberCap is one organization member with caps and live usage. Amounts are
// cents.
type MemberCap struct {
	UserID                 string `json:"user_id"`
	Email                  string `json:"email,omitempty"`
	Name                   string `json:"name,omitempty"`
	Role                   string `json:"role"`
	UsageCapCents          *int64 `json:"usage_cap_cents"`
	MaxConcurrentSandboxes *int   `json:"max_concurrent_sandboxes"`
	UsageCents             int64  `json:"usage_cents"`
	RunningSandboxes       int    `json:"running_sandboxes"`
}

// MemberCapList is every member with caps and the billing window their usage
// is counted in.
type MemberCapList struct {
	Members     []MemberCap `json:"members"`
	WindowStart string      `json:"window_start,omitempty"`
	WindowEnd   string      `json:"window_end,omitempty"`
}

// UnmarshalJSON accepts the object form {members, window_start, window_end}
// and a bare list of members.
func (l *MemberCapList) UnmarshalJSON(data []byte) error {
	var list []MemberCap
	if err := json.Unmarshal(data, &list); err == nil {
		*l = MemberCapList{Members: list}
		return nil
	}
	type plain MemberCapList
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*l = MemberCapList(p)
	return nil
}

// MemberCapsInput changes a member's caps. A nil pointer keeps the current
// value; the Clear flags remove a cap. At least one field must be set.
type MemberCapsInput struct {
	// UsageCapCents sets the usage cap (a positive number of cents).
	UsageCapCents *int64
	// ClearUsageCap removes the usage cap.
	ClearUsageCap bool
	// MaxConcurrentSandboxes sets the concurrency cap (a positive count).
	MaxConcurrentSandboxes *int
	// ClearMaxConcurrentSandboxes removes the concurrency cap.
	ClearMaxConcurrentSandboxes bool
}

// MarshalJSON implements json.Marshaler: a set cap is a number, a cleared cap
// is null, an untouched cap is absent.
func (in MemberCapsInput) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{}
	switch {
	case in.ClearUsageCap:
		m["usage_cap_cents"] = nil
	case in.UsageCapCents != nil:
		m["usage_cap_cents"] = *in.UsageCapCents
	}
	switch {
	case in.ClearMaxConcurrentSandboxes:
		m["max_concurrent_sandboxes"] = nil
	case in.MaxConcurrentSandboxes != nil:
		m["max_concurrent_sandboxes"] = *in.MaxConcurrentSandboxes
	}
	return json.Marshal(m)
}

// MemberCapsService manages per-member spend and concurrency caps. Owner and
// admin of the organization being viewed only. Accessed via Client.MemberCaps.
type MemberCapsService struct {
	client *Client
}

func memberCapsPath(tenantID string, parts ...string) string {
	p := "/tenants/" + url.PathEscape(tenantID) + "/member-caps"
	for _, part := range parts {
		p += "/" + url.PathEscape(part)
	}
	return p
}

// List returns every member of the organization with caps and live usage.
func (s *MemberCapsService) List(ctx context.Context, tenantID string) (*MemberCapList, error) {
	var out apiResponse[MemberCapList]
	if err := s.client.getJSON(ctx, memberCapsPath(tenantID), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Set changes a member's caps. Create and resume then answer
// 402 TEAM_MEMBER_CAP_REACHED or 429 MEMBER_LIMIT_REACHED for that member.
func (s *MemberCapsService) Set(ctx context.Context, tenantID, userID string, in MemberCapsInput) (*MemberCap, error) {
	if in.UsageCapCents == nil && !in.ClearUsageCap && in.MaxConcurrentSandboxes == nil && !in.ClearMaxConcurrentSandboxes {
		return nil, errors.New("send at least one cap")
	}
	var out apiResponse[MemberCap]
	if err := s.client.putJSON(ctx, memberCapsPath(tenantID, userID), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Clear removes both caps of a member.
func (s *MemberCapsService) Clear(ctx context.Context, tenantID, userID string) (*MemberCap, error) {
	var out apiResponse[MemberCap]
	if err := s.client.deleteJSON(ctx, memberCapsPath(tenantID, userID), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}
