package repository

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

type redeemCodeRepository struct {
	client *dbent.Client
}

func NewRedeemCodeRepository(client *dbent.Client) service.RedeemCodeRepository {
	return &redeemCodeRepository{client: client}
}

func (r *redeemCodeRepository) Create(ctx context.Context, code *service.RedeemCode) error {
	created, err := r.client.RedeemCode.Create().
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays).
		SetNillableExpiresAt(code.ExpiresAt).
		SetNillableUsedBy(code.UsedBy).
		SetNillableUsedAt(code.UsedAt).
		SetNillableGroupID(code.GroupID).
		Save(ctx)
	if err == nil {
		code.ID = created.ID
		code.CreatedAt = created.CreatedAt
	}
	return err
}

func (r *redeemCodeRepository) CreateBatch(ctx context.Context, codes []service.RedeemCode) error {
	if len(codes) == 0 {
		return nil
	}

	builders := make([]*dbent.RedeemCodeCreate, 0, len(codes))
	for i := range codes {
		c := &codes[i]
		b := r.client.RedeemCode.Create().
			SetCode(c.Code).
			SetType(c.Type).
			SetValue(c.Value).
			SetStatus(c.Status).
			SetNotes(c.Notes).
			SetValidityDays(c.ValidityDays).
			SetNillableExpiresAt(c.ExpiresAt).
			SetNillableUsedBy(c.UsedBy).
			SetNillableUsedAt(c.UsedAt).
			SetNillableGroupID(c.GroupID)
		builders = append(builders, b)
	}

	return r.client.RedeemCode.CreateBulk(builders...).Exec(ctx)
}

func (r *redeemCodeRepository) GetByID(ctx context.Context, id int64) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return redeemCodeEntityToService(m), nil
}

func (r *redeemCodeRepository) GetByCode(ctx context.Context, code string) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.CodeEQ(code)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return redeemCodeEntityToService(m), nil
}

func (r *redeemCodeRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.client.RedeemCode.Delete().Where(redeemcode.IDEQ(id)).Exec(ctx)
	return err
}

func (r *redeemCodeRepository) List(ctx context.Context, params pagination.PaginationParams) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	return r.ListWithFilters(ctx, params, "", "", "")
}

func (r *redeemCodeRepository) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	q := r.client.RedeemCode.Query()

	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	}
	if status != "" {
		now := time.Now()
		switch status {
		case service.StatusExpired:
			q = q.Where(redeemcode.Or(
				redeemcode.StatusEQ(service.StatusExpired),
				redeemcode.And(
					redeemcode.StatusEQ(service.StatusUnused),
					redeemcode.ExpiresAtNotNil(),
					redeemcode.ExpiresAtLTE(now),
				),
			))
		case service.StatusUnused:
			q = q.Where(
				redeemcode.StatusEQ(service.StatusUnused),
				redeemcode.Or(
					redeemcode.ExpiresAtIsNil(),
					redeemcode.ExpiresAtGT(now),
				),
			)
		default:
			q = q.Where(redeemcode.StatusEQ(status))
		}
	}
	if search != "" {
		q = q.Where(
			redeemcode.Or(
				redeemcode.CodeContainsFold(search),
				redeemcode.HasUserWith(user.EmailContainsFold(search)),
			),
		)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	codesQuery := q.
		WithUser().
		WithGroup().
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range redeemCodeListOrder(params) {
		codesQuery = codesQuery.Order(order)
	}

	codes, err := codesQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	outCodes := redeemCodeEntitiesToService(codes)

	return outCodes, paginationResultFromTotal(int64(total), params), nil
}

func redeemCodeListOrder(params pagination.PaginationParams) []func(*entsql.Selector) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	var field string
	switch sortBy {
	case "type":
		field = redeemcode.FieldType
	case "value":
		field = redeemcode.FieldValue
	case "status":
		field = redeemcode.FieldStatus
	case "used_at":
		field = redeemcode.FieldUsedAt
	case "created_at":
		field = redeemcode.FieldCreatedAt
	case "expires_at":
		field = redeemcode.FieldExpiresAt
	case "code":
		field = redeemcode.FieldCode
	default:
		field = redeemcode.FieldID
	}

	if sortOrder == pagination.SortOrderAsc {
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(redeemcode.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(redeemcode.FieldID)}
}

func (r *redeemCodeRepository) Update(ctx context.Context, code *service.RedeemCode) error {
	up := r.client.RedeemCode.UpdateOneID(code.ID).
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays)

	if code.UsedBy != nil {
		up.SetUsedBy(*code.UsedBy)
	} else {
		up.ClearUsedBy()
	}
	if code.UsedAt != nil {
		up.SetUsedAt(*code.UsedAt)
	} else {
		up.ClearUsedAt()
	}
	if code.GroupID != nil {
		up.SetGroupID(*code.GroupID)
	} else {
		up.ClearGroupID()
	}
	if code.ExpiresAt != nil {
		up.SetExpiresAt(*code.ExpiresAt)
	} else {
		up.ClearExpiresAt()
	}

	updated, err := up.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrRedeemCodeNotFound
		}
		return err
	}
	code.CreatedAt = updated.CreatedAt
	return nil
}

func (r *redeemCodeRepository) BatchUpdate(ctx context.Context, ids []int64, fields service.RedeemCodeBatchUpdateFields) (int64, error) {
	uniqueIDs := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return 0, nil
	}

	if tx := dbent.TxFromContext(ctx); tx != nil {
		return r.batchUpdate(ctx, tx.Client(), uniqueIDs, fields)
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	defer func() { _ = tx.Rollback() }()

	updated, err := r.batchUpdate(txCtx, tx.Client(), uniqueIDs, fields)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return updated, nil
}

func (r *redeemCodeRepository) batchUpdate(ctx context.Context, client *dbent.Client, ids []int64, fields service.RedeemCodeBatchUpdateFields) (int64, error) {
	existing, err := client.RedeemCode.Query().
		Where(redeemcode.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return 0, err
	}
	if len(existing) != len(ids) {
		return 0, service.ErrRedeemCodeNotFound
	}
	if fields.TouchesUsedSensitiveFields() {
		for _, code := range existing {
			if code.Status == service.StatusUsed {
				return 0, service.ErrRedeemCodeUsed
			}
		}
	}

	up := client.RedeemCode.Update().Where(redeemcode.IDIn(ids...))
	if fields.Status != nil {
		up.SetStatus(*fields.Status)
	}
	if fields.Notes != nil {
		up.SetNotes(*fields.Notes)
	}
	if fields.ExpiresAt.Set {
		if fields.ExpiresAt.Value != nil {
			up.SetExpiresAt(*fields.ExpiresAt.Value)
		} else {
			up.ClearExpiresAt()
		}
	}
	if fields.GroupID.Set {
		if fields.GroupID.Value != nil {
			up.SetGroupID(*fields.GroupID.Value)
		} else {
			up.ClearGroupID()
		}
	}

	affected, err := up.Save(ctx)
	if err != nil {
		return 0, err
	}
	if affected != len(ids) {
		return 0, service.ErrRedeemCodeNotFound
	}
	return int64(affected), nil
}

func (r *redeemCodeRepository) Use(ctx context.Context, id, userID int64) error {
	now := time.Now()
	client := clientFromContext(ctx, r.client)
	affected, err := client.RedeemCode.Update().
		Where(redeemcode.IDEQ(id), redeemcode.StatusEQ(service.StatusUnused)).
		SetStatus(service.StatusUsed).
		SetUsedBy(userID).
		SetUsedAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrRedeemCodeUsed
	}
	return nil
}

func (r *redeemCodeRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]service.RedeemCode, error) {
	if limit <= 0 {
		limit = 10
	}

	codes, err := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID)).
		WithGroup().
		Order(dbent.Desc(redeemcode.FieldUsedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return redeemCodeEntitiesToService(codes), nil
}

// ListByUserPaginated returns paginated balance/concurrency history for a user.
// Supports optional type filter (e.g. "balance", "admin_balance", "concurrency", "admin_concurrency", "subscription").
func (r *redeemCodeRepository) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	q := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID))

	// Optional type filter
	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	codes, err := q.
		WithGroup().
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(redeemcode.FieldUsedAt)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return redeemCodeEntitiesToService(codes), paginationResultFromTotal(int64(total), params), nil
}

// SumPositiveBalanceByUser returns total recharged amount (sum of value > 0 where type is balance/admin_balance).
func (r *redeemCodeRepository) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	var result []struct {
		Sum float64 `json:"sum"`
	}
	err := r.client.RedeemCode.Query().
		Where(
			redeemcode.UsedByEQ(userID),
			redeemcode.ValueGT(0),
			redeemcode.TypeIn("balance", "admin_balance"),
		).
		Aggregate(dbent.As(dbent.Sum(redeemcode.FieldValue), "sum")).
		Scan(ctx, &result)
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Sum, nil
}

func (r *redeemCodeRepository) GetCostCalculatorBalanceRechargeSummary(ctx context.Context, startTime, endTime time.Time, excludeAdmins bool) (*service.CostCalculatorBalanceRechargeSummary, error) {
	return r.GetCostCalculatorBalanceRechargeSummaryWithPackages(ctx, startTime, endTime, nil, excludeAdmins)
}

func (r *redeemCodeRepository) GetCostCalculatorBalanceRechargeSummaryWithPackages(ctx context.Context, startTime, endTime time.Time, packages []service.CostCalculatorBalanceRechargePackage, excludeAdmins bool) (*service.CostCalculatorBalanceRechargeSummary, error) {
	summary := &service.CostCalculatorBalanceRechargeSummary{
		PackageStats: []service.CostCalculatorBalanceRechargePackageStat{},
	}
	packageByAmount := make(map[int64]service.CostCalculatorBalanceRechargePackage, len(packages))
	packageStats := make(map[int64]*service.CostCalculatorBalanceRechargePackageStat, len(packages))
	for _, item := range packages {
		if item.BalanceAmount <= 0 || item.ActualAmount < 0 {
			continue
		}
		key := costCalculatorPackageKey(item.BalanceAmount)
		packageByAmount[key] = item
	}

	rows, err := r.queryCostCalculatorBalanceRechargeGroups(ctx, startTime, endTime, excludeAdmins)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		amount := row.Total
		summary.TotalAmount += amount
		summary.RecordCount += row.Count

		switch row.Type {
		case service.RedeemTypeBalance:
			summary.RedeemAmount += amount
			summary.RedeemCount += row.Count
		case service.AdjustmentTypeAdminBalance:
			summary.AdminNetAmount += amount
			summary.AdminCount += row.Count
			if row.Value > 0 {
				summary.AdminAddedAmount += amount
			} else if row.Value < 0 {
				summary.AdminDeductedAmount += -amount
			}
		}

		if row.Value <= 0 {
			continue
		}
		pkg, ok := packageByAmount[costCalculatorPackageKey(row.Value)]
		if !ok {
			summary.UnmatchedBalanceAmount += amount
			summary.UnmatchedRecordCount += row.Count
			continue
		}

		actualTotal := pkg.ActualAmount * float64(row.Count)
		summary.ActualRevenue += actualTotal
		summary.MatchedBalanceAmount += amount
		summary.MatchedRecordCount += row.Count
		if row.Type == service.RedeemTypeBalance {
			summary.RedeemActualRevenue += actualTotal
		} else if row.Type == service.AdjustmentTypeAdminBalance {
			summary.AdminActualRevenue += actualTotal
		}

		key := costCalculatorPackageKey(pkg.BalanceAmount)
		stat := packageStats[key]
		if stat == nil {
			stat = &service.CostCalculatorBalanceRechargePackageStat{
				BalanceAmount: pkg.BalanceAmount,
				ActualAmount:  pkg.ActualAmount,
			}
			packageStats[key] = stat
		}
		stat.Count += row.Count
		stat.BalanceTotal += amount
		stat.ActualTotal += actualTotal
	}

	for _, stat := range packageStats {
		summary.PackageStats = append(summary.PackageStats, *stat)
	}
	sort.Slice(summary.PackageStats, func(i, j int) bool {
		return summary.PackageStats[i].BalanceAmount < summary.PackageStats[j].BalanceAmount
	})

	rewardAmount, rewardCount, err := r.getCostCalculatorLeaderboardRewardSummary(ctx, startTime, endTime, excludeAdmins)
	if err != nil {
		return nil, err
	}
	summary.LeaderboardRewardAmount = rewardAmount
	summary.LeaderboardRewardCount = rewardCount
	return summary, nil
}

func (r *redeemCodeRepository) sumAndCountRedeemCodeValues(ctx context.Context, predicates ...predicate.RedeemCode) (float64, int64, error) {
	count, err := r.client.RedeemCode.Query().Where(predicates...).Count(ctx)
	if err != nil {
		return 0, 0, err
	}

	var result []struct {
		Sum sql.NullFloat64 `json:"sum"`
	}
	err = r.client.RedeemCode.Query().
		Where(predicates...).
		Aggregate(dbent.As(dbent.Sum(redeemcode.FieldValue), "sum")).
		Scan(ctx, &result)
	if err != nil {
		return 0, 0, err
	}
	if len(result) == 0 || !result[0].Sum.Valid {
		return 0, int64(count), nil
	}
	return result[0].Sum.Float64, int64(count), nil
}

type costCalculatorBalanceRechargeGroup struct {
	Type  string
	Value float64
	Total float64
	Count int64
}

func (r *redeemCodeRepository) queryCostCalculatorBalanceRechargeGroups(ctx context.Context, startTime, endTime time.Time, excludeAdmins bool) ([]costCalculatorBalanceRechargeGroup, error) {
	args := []any{service.StatusUsed, startTime, endTime, service.RedeemTypeBalance, service.AdjustmentTypeAdminBalance}
	query := `
SELECT rc.type, rc.value::double precision, COUNT(*)::bigint, COALESCE(SUM(rc.value), 0)::double precision
FROM redeem_codes rc
WHERE rc.status = $1
  AND rc.used_at IS NOT NULL
  AND rc.used_at >= $2
  AND rc.used_at < $3
  AND rc.type IN ($4, $5)`
	if excludeAdmins {
		args = append(args, service.RoleAdmin)
		query += `
  AND EXISTS (
    SELECT 1 FROM users cost_calc_u
    WHERE cost_calc_u.id = rc.used_by AND cost_calc_u.role <> $6
  )`
	}
	query += `
GROUP BY rc.type, rc.value
ORDER BY rc.type, rc.value`

	rows, err := r.client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	results := make([]costCalculatorBalanceRechargeGroup, 0)
	for rows.Next() {
		var row costCalculatorBalanceRechargeGroup
		var total sql.NullFloat64
		if err := rows.Scan(&row.Type, &row.Value, &row.Count, &total); err != nil {
			return nil, err
		}
		if total.Valid {
			row.Total = total.Float64
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *redeemCodeRepository) getCostCalculatorLeaderboardRewardSummary(ctx context.Context, startTime, endTime time.Time, excludeAdmins bool) (float64, int64, error) {
	args := []any{startTime, endTime}
	query := `
SELECT COUNT(*)::bigint, COALESCE(SUM(l.reward_amount), 0)::double precision
FROM leaderboard_reward_logs l
WHERE l.created_at >= $1 AND l.created_at < $2`
	if excludeAdmins {
		args = append(args, service.RoleAdmin)
		query += `
  AND EXISTS (
    SELECT 1 FROM users cost_calc_u
    WHERE cost_calc_u.id = l.user_id AND cost_calc_u.role <> $3
  )`
	}

	rows, err := r.client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()

	var count int64
	var amount sql.NullFloat64
	if rows.Next() {
		if err := rows.Scan(&count, &amount); err != nil {
			return 0, 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	if !amount.Valid {
		return 0, count, nil
	}
	return amount.Float64, count, nil
}

func costCalculatorPackageKey(value float64) int64 {
	return int64(value*1_000_000 + 0.5)
}

func redeemCodeEntityToService(m *dbent.RedeemCode) *service.RedeemCode {
	if m == nil {
		return nil
	}
	out := &service.RedeemCode{
		ID:           m.ID,
		Code:         m.Code,
		Type:         m.Type,
		Value:        m.Value,
		Status:       m.Status,
		UsedBy:       m.UsedBy,
		UsedAt:       m.UsedAt,
		Notes:        derefString(m.Notes),
		CreatedAt:    m.CreatedAt,
		ExpiresAt:    m.ExpiresAt,
		GroupID:      m.GroupID,
		ValidityDays: m.ValidityDays,
	}
	if m.Edges.User != nil {
		out.User = userEntityToService(m.Edges.User)
	}
	if m.Edges.Group != nil {
		out.Group = groupEntityToService(m.Edges.Group)
	}
	return out
}

func redeemCodeEntitiesToService(models []*dbent.RedeemCode) []service.RedeemCode {
	out := make([]service.RedeemCode, 0, len(models))
	for i := range models {
		if s := redeemCodeEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}
