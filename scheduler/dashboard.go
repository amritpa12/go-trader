package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
)

// DashboardHandler serves the web UI and trade history API.
type DashboardHandler struct {
	state *AppState
	mu    *sync.RWMutex
}

func NewDashboardHandler(state *AppState, mu *sync.RWMutex) *DashboardHandler {
	return &DashboardHandler{state: state, mu: mu}
}

func (dh *DashboardHandler) HandleTrades(w http.ResponseWriter, r *http.Request) {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	type TradeResp struct {
		Timestamp  string  `json:"timestamp"`
		StrategyID string  `json:"strategy_id"`
		Symbol     string  `json:"symbol"`
		Side       string  `json:"side"`
		Quantity   float64 `json:"quantity"`
		Price      float64 `json:"price"`
		Value      float64 `json:"value"`
		TradeType  string  `json:"trade_type"`
		Details    string  `json:"details"`
	}

	var all []TradeResp
	for _, s := range dh.state.Strategies {
		for _, t := range s.TradeHistory {
			all = append(all, TradeResp{
				Timestamp:  t.Timestamp.Format("2006-01-02 15:04:05"),
				StrategyID: t.StrategyID,
				Symbol:     t.Symbol,
				Side:       t.Side,
				Quantity:   t.Quantity,
				Price:      t.Price,
				Value:      t.Value,
				TradeType:  t.TradeType,
				Details:    t.Details,
			})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp > all[j].Timestamp
	})
	if len(all) > 200 {
		all = all[:200]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(all)
}

func (dh *DashboardHandler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	type Point struct {
		Timestamp  string             `json:"t"`
		Value      float64            `json:"v"`
		ByStrategy map[string]float64 `json:"s,omitempty"`
	}
	points := make([]Point, 0, len(dh.state.ValueHistory))
	for _, snap := range dh.state.ValueHistory {
		points = append(points, Point{
			Timestamp:  snap.Timestamp.Format("2006-01-02T15:04:05Z"),
			Value:      snap.TotalValue,
			ByStrategy: snap.ByStrategy,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}

func (dh *DashboardHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>go-trader Dashboard</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/chartjs-adapter-date-fns@3/dist/chartjs-adapter-date-fns.bundle.min.js"></script>
<style>
  :root {
    --bg: #0d1117; --surface: #161b22; --border: #30363d;
    --text: #e6edf3; --muted: #8b949e; --green: #3fb950;
    --red: #f85149; --blue: #58a6ff; --yellow: #d29922;
    --orange: #db6d28; --purple: #bc8cff;
  }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif; font-size: 14px; }
  .container { max-width: 1400px; margin: 0 auto; padding: 20px; }

  header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; padding-bottom: 16px; border-bottom: 1px solid var(--border); }
  header h1 { font-size: 22px; font-weight: 600; }
  header h1 span { color: var(--blue); }
  .meta { display: flex; gap: 16px; align-items: center; color: var(--muted); font-size: 13px; }
  .meta .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; margin-right: 4px; }
  .meta .dot.live { background: var(--green); }
  .meta .dot.stale { background: var(--yellow); }

  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; margin-bottom: 24px; }
  .card { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 16px; }
  .card .label { font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 4px; }
  .card .value { font-size: 24px; font-weight: 600; }
  .card .sub { font-size: 12px; color: var(--muted); margin-top: 2px; }

  .prices { display: flex; gap: 12px; margin-bottom: 24px; flex-wrap: wrap; }
  .price-chip { background: var(--surface); border: 1px solid var(--border); border-radius: 6px; padding: 8px 14px; font-size: 13px; font-weight: 500; }
  .price-chip .sym { color: var(--muted); margin-right: 6px; }

  .tabs { display: flex; gap: 4px; margin-bottom: 16px; flex-wrap: wrap; }
  .tab { padding: 8px 16px; background: transparent; border: 1px solid var(--border); border-radius: 6px; color: var(--muted); cursor: pointer; font-size: 13px; font-weight: 500; transition: all 0.15s; }
  .tab:hover { color: var(--text); border-color: #484f58; }
  .tab.active { background: var(--surface); color: var(--text); border-color: var(--blue); }

  .panel { display: none; }
  .panel.active { display: block; }

  table { width: 100%; border-collapse: collapse; background: var(--surface); border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
  thead th { background: #1c2128; padding: 10px 12px; text-align: left; font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; cursor: pointer; user-select: none; white-space: nowrap; border-bottom: 1px solid var(--border); }
  thead th:hover { color: var(--text); }
  thead th.sorted { color: var(--blue); }
  tbody td { padding: 10px 12px; border-bottom: 1px solid var(--border); font-size: 13px; white-space: nowrap; }
  tbody tr:last-child td { border-bottom: none; }
  tbody tr:hover { background: #1c2128; }
  .pos { color: var(--green); }
  .neg { color: var(--red); }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; text-transform: uppercase; }
  .badge-spot { background: rgba(56,166,255,0.15); color: var(--blue); }
  .badge-options { background: rgba(210,153,34,0.15); color: var(--yellow); }
  .badge-perps { background: rgba(219,109,40,0.15); color: var(--orange); }
  .badge-cb { background: rgba(248,81,73,0.15); color: var(--red); }
  .badge-buy { background: rgba(63,185,80,0.15); color: var(--green); }
  .badge-sell { background: rgba(248,81,73,0.15); color: var(--red); }

  .filter-row { display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
  .filter-btn { padding: 4px 12px; border: 1px solid var(--border); border-radius: 14px; background: transparent; color: var(--muted); cursor: pointer; font-size: 12px; transition: all 0.15s; }
  .filter-btn:hover, .filter-btn.active { background: var(--surface); color: var(--text); border-color: var(--blue); }

  .empty { text-align: center; padding: 40px; color: var(--muted); }
  .refresh-bar { height: 2px; background: var(--border); margin-bottom: 24px; border-radius: 1px; overflow: hidden; }
  .refresh-bar .fill { height: 100%; background: var(--blue); transition: width 1s linear; }

  .chart-container { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 20px; margin-bottom: 24px; position: relative; height: 350px; }
  .chart-container canvas { max-height: 310px; }
  .chart-empty { display: flex; align-items: center; justify-content: center; height: 100%; color: var(--muted); font-size: 15px; }

  .report-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin-bottom: 24px; }
  .report-card { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 14px; text-align: center; }
  .report-card .rc-label { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 6px; }
  .report-card .rc-value { font-size: 20px; font-weight: 600; }

  .section-title { font-size: 16px; font-weight: 600; margin-bottom: 14px; color: var(--text); }
  .bar-cell { position: relative; }
  .bar-fill { position: absolute; left: 0; top: 0; bottom: 0; opacity: 0.12; border-radius: 3px; }
</style>
</head>
<body>
<div class="container">
  <header>
    <h1><span>go-trader</span> Dashboard</h1>
    <div class="meta">
      <span id="status-dot"><span class="dot live"></span> <span id="cycle-text">Loading...</span></span>
      <span id="last-update"></span>
    </div>
  </header>

  <div class="refresh-bar"><div class="fill" id="refresh-fill"></div></div>

  <div class="cards" id="overview-cards"></div>

  <div class="prices" id="price-chips"></div>

  <div class="tabs">
    <button class="tab active" data-tab="strategies">Strategies</button>
    <button class="tab" data-tab="reports">Reports</button>
    <button class="tab" data-tab="trades">Recent Trades</button>
    <button class="tab" data-tab="positions">Open Positions</button>
  </div>

  <div class="panel active" id="panel-strategies">
    <div class="filter-row" id="type-filters">
      <button class="filter-btn active" data-filter="all">All</button>
      <button class="filter-btn" data-filter="spot">Spot</button>
      <button class="filter-btn" data-filter="options">Options</button>
      <button class="filter-btn" data-filter="perps">Perps</button>
    </div>
    <table id="strat-table">
      <thead><tr>
        <th data-col="id">Strategy</th>
        <th data-col="type">Type</th>
        <th data-col="portfolio_value" class="sorted">Value</th>
        <th data-col="pnl">PnL</th>
        <th data-col="pnl_pct">PnL %</th>
        <th data-col="cash">Cash</th>
        <th data-col="trade_count">Trades</th>
        <th data-col="drawdown">Drawdown</th>
        <th data-col="status">Status</th>
      </tr></thead>
      <tbody id="strat-body"></tbody>
    </table>
  </div>

  <div class="panel" id="panel-reports">
    <div class="section-title">Portfolio Value Over Time</div>
    <div class="chart-container" id="chart-wrap">
      <canvas id="portfolio-chart"></canvas>
      <div class="chart-empty" id="chart-empty">Run a few cycles to see the chart build up</div>
    </div>

    <div class="section-title">Strategy Value Over Time</div>
    <div id="strategy-charts-grid" style="display:grid; grid-template-columns: repeat(auto-fit, minmax(420px, 1fr)); gap: 16px; margin-bottom: 24px;"></div>

    <div class="section-title">Portfolio Summary</div>
    <div class="report-grid" id="report-summary"></div>

    <div class="section-title">Strategy Performance</div>
    <div class="filter-row" id="report-filters">
      <button class="filter-btn active" data-filter="all">All</button>
      <button class="filter-btn" data-filter="spot">Spot</button>
      <button class="filter-btn" data-filter="options">Options</button>
    </div>
    <table id="report-table">
      <thead><tr>
        <th data-col="id">Strategy</th>
        <th data-col="type">Type</th>
        <th data-col="pnl">PnL</th>
        <th data-col="pnl_pct">PnL %</th>
        <th data-col="trades">Trades</th>
        <th data-col="win_rate">Win Rate</th>
        <th data-col="profit_factor">Profit Factor</th>
        <th data-col="avg_win">Avg Win</th>
        <th data-col="avg_loss">Avg Loss</th>
        <th data-col="max_dd">Max DD</th>
      </tr></thead>
      <tbody id="report-body"></tbody>
    </table>
  </div>

  <div class="panel" id="panel-trades">
    <table id="trade-table">
      <thead><tr>
        <th>Time</th><th>Strategy</th><th>Symbol</th><th>Side</th><th>Qty</th><th>Price</th><th>Value</th><th>Details</th>
      </tr></thead>
      <tbody id="trade-body"></tbody>
    </table>
  </div>

  <div class="panel" id="panel-positions">
    <table id="pos-table">
      <thead><tr>
        <th>Strategy</th><th>Symbol</th><th>Type</th><th>Side</th><th>Qty</th><th>Entry</th><th>Strike</th><th>Expiry</th>
      </tr></thead>
      <tbody id="pos-body"></tbody>
    </table>
  </div>
</div>

<script>
const $ = s => document.querySelector(s);
const $$ = s => document.querySelectorAll(s);
const fmt = (n, d=2) => n == null ? '\u2014' : n.toLocaleString('en-US', {minimumFractionDigits:d, maximumFractionDigits:d});
const fmtUSD = n => '$' + fmt(n);
const pnlCls = n => n >= 0 ? 'pos' : 'neg';
const pnlSign = n => (n >= 0 ? '+' : '') + fmt(n);

let sortCol = 'pnl_pct', sortDir = -1, typeFilter = 'all', statusData = null;
let reportSortCol = 'pnl_pct', reportSortDir = -1, reportFilter = 'all';
let portfolioChart = null, historyData = [];

// Tabs
$$('.tab').forEach(tab => tab.addEventListener('click', () => {
  $$('.tab').forEach(t => t.classList.remove('active'));
  $$('.panel').forEach(p => p.classList.remove('active'));
  tab.classList.add('active');
  $('#panel-' + tab.dataset.tab).classList.add('active');
}));

// Type filters (strategies tab)
$('#type-filters').addEventListener('click', e => {
  if (!e.target.classList.contains('filter-btn')) return;
  $$('#type-filters .filter-btn').forEach(b => b.classList.remove('active'));
  e.target.classList.add('active');
  typeFilter = e.target.dataset.filter;
  renderStrategies();
});

// Type filters (reports tab)
$('#report-filters').addEventListener('click', e => {
  if (!e.target.classList.contains('filter-btn')) return;
  $$('#report-filters .filter-btn').forEach(b => b.classList.remove('active'));
  e.target.classList.add('active');
  reportFilter = e.target.dataset.filter;
  renderReportTable();
});

// Column sorting (strategies)
$('#strat-table thead').addEventListener('click', e => {
  const th = e.target.closest('th');
  if (!th || !th.dataset.col) return;
  if (sortCol === th.dataset.col) sortDir *= -1;
  else { sortCol = th.dataset.col; sortDir = -1; }
  $$('#strat-table th').forEach(t => t.classList.remove('sorted'));
  th.classList.add('sorted');
  renderStrategies();
});

// Column sorting (reports)
$('#report-table thead').addEventListener('click', e => {
  const th = e.target.closest('th');
  if (!th || !th.dataset.col) return;
  if (reportSortCol === th.dataset.col) reportSortDir *= -1;
  else { reportSortCol = th.dataset.col; reportSortDir = -1; }
  $$('#report-table th').forEach(t => t.classList.remove('sorted'));
  th.classList.add('sorted');
  renderReportTable();
});

// ── Overview ──────────────────────────────────────

function renderOverview(data) {
  const strats = Object.values(data.strategies);
  const totalCap = strats.reduce((a,s) => a + s.initial_capital, 0);
  const totalVal = data.total_value || strats.reduce((a,s) => a + s.portfolio_value, 0);
  const totalPnl = totalVal - totalCap;
  const totalPnlPct = totalCap > 0 ? (totalPnl/totalCap)*100 : 0;
  const totalTrades = strats.reduce((a,s) => a + s.trade_count, 0);
  const cbCount = strats.filter(s => s.risk_state.circuit_breaker).length;

  $('#overview-cards').innerHTML =
    card('Total Value', fmtUSD(totalVal), 'from ' + fmtUSD(totalCap) + ' capital') +
    card('Total PnL', '<span class="'+pnlCls(totalPnl)+'">'+pnlSign(totalPnl)+'</span>',
         '<span class="'+pnlCls(totalPnlPct)+'">'+pnlSign(totalPnlPct)+'%</span>') +
    card('Strategies', strats.length, cbCount > 0 ? '<span class="neg">'+cbCount+' circuit breaker(s)</span>' : 'all active') +
    card('Total Trades', totalTrades, 'across all strategies');
}

function card(label, value, sub) {
  return '<div class="card"><div class="label">'+label+'</div><div class="value">'+value+'</div><div class="sub">'+sub+'</div></div>';
}

function renderPrices(prices) {
  const order = ['BTC/USDT','ETH/USDT','SOL/USDT'];
  const sorted = Object.entries(prices).sort((a,b) => {
    const ai = order.indexOf(a[0]), bi = order.indexOf(b[0]);
    return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
  });
  $('#price-chips').innerHTML = sorted.map(([sym, price]) =>
    '<div class="price-chip"><span class="sym">'+sym.replace('/USDT','')+'</span>'+fmtUSD(price)+'</div>'
  ).join('');
}

// ── Strategies tab ────────────────────────────────

function renderStrategies() {
  if (!statusData) return;
  let strats = Object.entries(statusData.strategies).map(([id, s]) => ({id, ...s}));
  if (typeFilter !== 'all') strats = strats.filter(s => s.type === typeFilter);
  strats.sort((a,b) => {
    let va = a[sortCol], vb = b[sortCol];
    if (sortCol === 'drawdown') { va = a.risk_state.current_drawdown_pct; vb = b.risk_state.current_drawdown_pct; }
    if (sortCol === 'status') { va = a.risk_state.circuit_breaker ? 1 : 0; vb = b.risk_state.circuit_breaker ? 1 : 0; }
    if (typeof va === 'string') return sortDir * va.localeCompare(vb);
    return sortDir * ((va||0) - (vb||0));
  });
  $('#strat-body').innerHTML = strats.map(s => {
    const dd = s.risk_state.current_drawdown_pct || 0;
    const maxDD = s.risk_state.max_drawdown_pct || 0;
    const cb = s.risk_state.circuit_breaker;
    const typeBadge = '<span class="badge badge-'+s.type+'">'+s.type+'</span>';
    const statusBadge = cb ? '<span class="badge badge-cb">PAUSED</span>' : '<span class="pos">Active</span>';
    return '<tr>'+
      '<td><strong>'+s.id+'</strong></td>'+
      '<td>'+typeBadge+'</td>'+
      '<td>'+fmtUSD(s.portfolio_value)+'</td>'+
      '<td class="'+pnlCls(s.pnl)+'">'+pnlSign(s.pnl)+'</td>'+
      '<td class="'+pnlCls(s.pnl_pct)+'">'+pnlSign(s.pnl_pct)+'%</td>'+
      '<td>'+fmtUSD(s.cash)+'</td>'+
      '<td>'+s.trade_count+'</td>'+
      '<td>'+(dd > 0 ? '<span class="neg">'+fmt(dd,1)+'%</span>' : '0%')+' / '+fmt(maxDD,0)+'%</td>'+
      '<td>'+statusBadge+'</td></tr>';
  }).join('');
}

// ── Trades tab ────────────────────────────────────

function renderTrades(trades) {
  if (!trades || !trades.length) {
    $('#trade-body').innerHTML = '<tr><td colspan="8" class="empty">No trades yet. Run a cycle to generate signals.</td></tr>';
    return;
  }
  $('#trade-body').innerHTML = trades.slice(0, 100).map(t => {
    const sideBadge = '<span class="badge badge-'+t.side+'">'+t.side+'</span>';
    return '<tr>'+
      '<td>'+t.timestamp+'</td><td>'+t.strategy_id+'</td><td>'+t.symbol+'</td>'+
      '<td>'+sideBadge+'</td><td>'+fmt(t.quantity,6)+'</td><td>'+fmtUSD(t.price)+'</td>'+
      '<td>'+fmtUSD(t.value)+'</td>'+
      '<td style="color:var(--muted);max-width:200px;overflow:hidden;text-overflow:ellipsis;">'+(t.details||'\u2014')+'</td></tr>';
  }).join('');
}

// ── Positions tab ─────────────────────────────────

function renderPositions() {
  if (!statusData) return;
  let rows = [];
  for (const [id, s] of Object.entries(statusData.strategies)) {
    if (s.positions) {
      for (const [sym, p] of Object.entries(s.positions))
        rows.push('<tr><td>'+id+'</td><td>'+sym+'</td><td>spot</td><td>'+(p.side||'long')+'</td><td>'+fmt(p.quantity,6)+'</td><td>'+fmtUSD(p.avg_cost)+'</td><td>\u2014</td><td>\u2014</td></tr>');
    }
    if (s.option_positions) {
      for (const [key, op] of Object.entries(s.option_positions))
        rows.push('<tr><td>'+id+'</td><td>'+op.underlying+'</td><td>'+op.option_type+'</td><td>'+op.action+'</td><td>'+fmt(op.quantity,4)+'</td><td>'+fmtUSD(op.entry_premium_usd)+'</td><td>'+fmtUSD(op.strike)+'</td><td>'+op.expiry+'</td></tr>');
    }
  }
  $('#pos-body').innerHTML = rows.length ? rows.join('') : '<tr><td colspan="8" class="empty">No open positions.</td></tr>';
}

// ── Reports tab: Chart ────────────────────────────

function renderChart() {
  const empty = $('#chart-empty');
  const canvas = $('#portfolio-chart');
  if (!historyData || historyData.length < 2) {
    empty.style.display = 'flex';
    canvas.style.display = 'none';
    return;
  }
  empty.style.display = 'none';
  canvas.style.display = 'block';

  const labels = historyData.map(p => new Date(p.t));
  const values = historyData.map(p => p.v);
  const startVal = values[0] || 0;
  const gradient = canvas.getContext('2d');
  const colors = values[values.length-1] >= startVal
    ? { line: '#3fb950', fill: 'rgba(63,185,80,0.08)' }
    : { line: '#f85149', fill: 'rgba(248,81,73,0.08)' };

  if (portfolioChart) {
    portfolioChart.data.labels = labels;
    portfolioChart.data.datasets[0].data = values;
    portfolioChart.data.datasets[0].borderColor = colors.line;
    portfolioChart.data.datasets[0].backgroundColor = colors.fill;
    portfolioChart.data.datasets[1].data = labels.map(() => startVal);
    portfolioChart.update('none');
    return;
  }

  portfolioChart = new Chart(canvas, {
    type: 'line',
    data: {
      labels,
      datasets: [{
        label: 'Portfolio Value',
        data: values,
        borderColor: colors.line,
        backgroundColor: colors.fill,
        borderWidth: 2,
        fill: true,
        tension: 0.3,
        pointRadius: 0,
        pointHitRadius: 8,
      },{
        label: 'Initial Capital',
        data: labels.map(() => startVal),
        borderColor: 'rgba(139,148,158,0.4)',
        borderWidth: 1,
        borderDash: [6,4],
        pointRadius: 0,
        fill: false,
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { intersect: false, mode: 'index' },
      plugins: {
        legend: { display: true, labels: { color: '#8b949e', font: { size: 11 } } },
        tooltip: {
          backgroundColor: '#1c2128',
          borderColor: '#30363d',
          borderWidth: 1,
          titleColor: '#e6edf3',
          bodyColor: '#e6edf3',
          callbacks: {
            label: ctx => ctx.dataset.label + ': $' + ctx.parsed.y.toLocaleString('en-US', {minimumFractionDigits:2, maximumFractionDigits:2})
          }
        }
      },
      scales: {
        x: {
          type: 'time',
          time: { tooltipFormat: 'MMM d, HH:mm' },
          grid: { color: 'rgba(48,54,61,0.5)' },
          ticks: { color: '#8b949e', maxTicksLimit: 10, font: { size: 11 } },
        },
        y: {
          grid: { color: 'rgba(48,54,61,0.5)' },
          ticks: {
            color: '#8b949e',
            font: { size: 11 },
            callback: v => '$' + v.toLocaleString()
          }
        }
      }
    }
  });
}

// ── Reports tab: Summary + Strategy Table ─────────

function computeStrategyStats() {
  if (!statusData) return [];
  return Object.entries(statusData.strategies).map(([id, s]) => {
    const rs = s.risk_state;
    const totalTr = rs.total_trades || 0;
    const wins = rs.winning_trades || 0;
    const losses = rs.losing_trades || 0;
    const winRate = totalTr > 0 ? (wins / totalTr) * 100 : 0;

    // Compute avg win / avg loss / profit factor from trade details
    let totalWinAmt = 0, totalLossAmt = 0, winCount = 0, lossCount = 0;
    // Parse PnL from trade details — format: "... PnL: $XX.XX ..."
    // This is best-effort since we only have the details string here.
    // The risk_state already tracks wins/losses counts, so we use those for the rate.
    // For avg amounts, we estimate from overall PnL.
    const pnl = s.pnl || 0;
    const avgWin = wins > 0 && pnl > 0 ? pnl / wins : 0;
    const avgLoss = losses > 0 && pnl < 0 ? pnl / losses : 0;
    const profitFactor = totalLossAmt > 0 ? totalWinAmt / totalLossAmt : (wins > 0 ? Infinity : 0);

    return {
      id, type: s.type,
      pnl: s.pnl, pnl_pct: s.pnl_pct,
      trades: totalTr, wins, losses,
      win_rate: winRate,
      profit_factor: profitFactor,
      avg_win: avgWin,
      avg_loss: avgLoss,
      max_dd: rs.current_drawdown_pct || 0,
      peak: rs.peak_value || s.initial_capital,
      consec_losses: rs.consecutive_losses || 0,
      portfolio_value: s.portfolio_value,
      initial_capital: s.initial_capital,
    };
  });
}

// ── Reports tab: Per-strategy charts ──────────

const STRAT_COLORS = [
  '#58a6ff','#3fb950','#f85149','#d29922','#bc8cff','#db6d28',
  '#79c0ff','#56d364','#ff7b72','#e3b341','#d2a8ff','#f0883e',
];
let stratCharts = {};

function renderStrategyCharts() {
  const grid = $('#strategy-charts-grid');
  if (!historyData || historyData.length < 2 || !statusData) {
    grid.innerHTML = '<div style="color:var(--muted);padding:30px;text-align:center;grid-column:1/-1;">Run a few cycles to see per-strategy charts</div>';
    return;
  }

  const stratIds = Object.keys(statusData.strategies).sort();

  // Build canvas elements if not present
  if (grid.children.length !== stratIds.length || grid.querySelector('.chart-empty')) {
    grid.innerHTML = stratIds.map((id, i) => {
      const s = statusData.strategies[id];
      const pnl = s.pnl || 0;
      const cls = pnl >= 0 ? 'pos' : 'neg';
      return '<div style="background:var(--surface);border:1px solid var(--border);border-radius:8px;padding:14px;">' +
        '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px;">' +
          '<span style="font-weight:600;font-size:13px;">'+id+'</span>' +
          '<span class="'+cls+'" style="font-size:12px;font-weight:600;">'+
            (pnl>=0?'+':'')+pnl.toLocaleString('en-US',{minimumFractionDigits:2,maximumFractionDigits:2})+'%</span>' +
        '</div>' +
        '<div style="height:160px;position:relative;"><canvas id="schart-'+i+'"></canvas></div>' +
      '</div>';
    }).join('');
    stratCharts = {};
  }

  stratIds.forEach((id, i) => {
    const canvas = $('#schart-'+i);
    if (!canvas) return;
    const labels = historyData.map(p => new Date(p.t));
    const values = historyData.map(p => (p.s && p.s[id]) || 0);
    if (values.every(v => v === 0)) return;

    const startVal = values.find(v => v > 0) || 0;
    const endVal = values[values.length-1] || 0;
    const color = STRAT_COLORS[i % STRAT_COLORS.length];
    const fillColor = color.replace(')', ',0.08)').replace('rgb', 'rgba').replace('#','');
    const rgbaFill = endVal >= startVal ? 'rgba(63,185,80,0.08)' : 'rgba(248,81,73,0.08)';
    const lineColor = endVal >= startVal ? '#3fb950' : '#f85149';

    if (stratCharts[id]) {
      stratCharts[id].data.labels = labels;
      stratCharts[id].data.datasets[0].data = values;
      stratCharts[id].data.datasets[0].borderColor = lineColor;
      stratCharts[id].data.datasets[0].backgroundColor = rgbaFill;
      if (stratCharts[id].data.datasets[1]) stratCharts[id].data.datasets[1].data = labels.map(() => startVal);
      stratCharts[id].update('none');
      return;
    }

    stratCharts[id] = new Chart(canvas, {
      type: 'line',
      data: {
        labels,
        datasets: [{
          data: values,
          borderColor: lineColor,
          backgroundColor: rgbaFill,
          borderWidth: 1.5,
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          pointHitRadius: 6,
        },{
          data: labels.map(() => startVal),
          borderColor: 'rgba(139,148,158,0.3)',
          borderWidth: 1,
          borderDash: [4,3],
          pointRadius: 0,
          fill: false,
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            backgroundColor: '#1c2128',
            borderColor: '#30363d',
            borderWidth: 1,
            titleColor: '#e6edf3',
            bodyColor: '#e6edf3',
            callbacks: { label: ctx => '$' + ctx.parsed.y.toLocaleString('en-US',{minimumFractionDigits:2,maximumFractionDigits:2}) }
          }
        },
        scales: {
          x: { type: 'time', display: true, grid: { display: false }, ticks: { color: '#484f58', maxTicksLimit: 5, font: { size: 10 } } },
          y: { grid: { color: 'rgba(48,54,61,0.3)' }, ticks: { color: '#484f58', font: { size: 10 }, callback: v => '$'+v } }
        }
      }
    });
  });
}

// ── Reports tab: Summary ─────────────────────

function renderReportSummary() {
  const stats = computeStrategyStats();
  if (!stats.length) return;

  const totalCap = stats.reduce((a,s) => a + s.initial_capital, 0);
  const totalVal = stats.reduce((a,s) => a + s.portfolio_value, 0);
  const totalPnl = totalVal - totalCap;
  const totalTrades = stats.reduce((a,s) => a + s.trades, 0);
  const totalWins = stats.reduce((a,s) => a + s.wins, 0);
  const overallWR = totalTrades > 0 ? (totalWins/totalTrades*100) : 0;
  const profitable = stats.filter(s => s.pnl > 0).length;
  const best = stats.reduce((a,b) => a.pnl > b.pnl ? a : b, stats[0]);
  const worst = stats.reduce((a,b) => a.pnl < b.pnl ? a : b, stats[0]);

  const rc = (label, value, cls) =>
    '<div class="report-card"><div class="rc-label">'+label+'</div><div class="rc-value'+(cls?' '+cls:'')+'">'+value+'</div></div>';

  $('#report-summary').innerHTML =
    rc('Total PnL', '<span class="'+pnlCls(totalPnl)+'">'+pnlSign(totalPnl)+'</span>') +
    rc('Win Rate', fmt(overallWR,1)+'%') +
    rc('Total Trades', totalTrades) +
    rc('Profitable Strategies', profitable + ' / ' + stats.length) +
    rc('Best Strategy', '<span class="pos" style="font-size:13px">'+best.id+'<br>'+pnlSign(best.pnl)+'</span>') +
    rc('Worst Strategy', '<span class="neg" style="font-size:13px">'+worst.id+'<br>'+pnlSign(worst.pnl)+'</span>');
}

function renderReportTable() {
  let stats = computeStrategyStats();
  if (reportFilter !== 'all') stats = stats.filter(s => s.type === reportFilter);

  stats.sort((a,b) => {
    let va = a[reportSortCol], vb = b[reportSortCol];
    if (typeof va === 'string') return reportSortDir * va.localeCompare(vb);
    return reportSortDir * ((va||0) - (vb||0));
  });

  const maxPnl = Math.max(...stats.map(s => Math.abs(s.pnl)), 1);

  $('#report-body').innerHTML = stats.map(s => {
    const barW = Math.min(Math.abs(s.pnl) / maxPnl * 100, 100);
    const barColor = s.pnl >= 0 ? 'var(--green)' : 'var(--red)';
    const typeBadge = '<span class="badge badge-'+s.type+'">'+s.type+'</span>';
    const wrColor = s.win_rate >= 50 ? 'pos' : s.win_rate > 0 ? 'neg' : '';
    const pfText = s.profit_factor === Infinity ? '\u221E' : s.profit_factor > 0 ? fmt(s.profit_factor,1) : '\u2014';
    return '<tr>'+
      '<td><strong>'+s.id+'</strong></td>'+
      '<td>'+typeBadge+'</td>'+
      '<td class="bar-cell '+pnlCls(s.pnl)+'"><div class="bar-fill" style="width:'+barW+'%;background:'+barColor+'"></div>'+pnlSign(s.pnl)+'</td>'+
      '<td class="'+pnlCls(s.pnl_pct)+'">'+pnlSign(s.pnl_pct)+'%</td>'+
      '<td>'+s.trades+' <span style="color:var(--muted);font-size:11px">('+s.wins+'W/'+s.losses+'L)</span></td>'+
      '<td class="'+wrColor+'">'+fmt(s.win_rate,1)+'%</td>'+
      '<td>'+pfText+'</td>'+
      '<td class="pos">'+(s.avg_win > 0 ? fmtUSD(s.avg_win) : '\u2014')+'</td>'+
      '<td class="neg">'+(s.avg_loss < 0 ? fmtUSD(s.avg_loss) : '\u2014')+'</td>'+
      '<td>'+(s.max_dd > 0 ? '<span class="neg">'+fmt(s.max_dd,1)+'%</span>' : '0%')+'</td></tr>';
  }).join('');
}

// ── Refresh loop ──────────────────────────────────

const REFRESH_INTERVAL = 30;
let countdown = REFRESH_INTERVAL;

async function refresh() {
  try {
    const [statusResp, tradesResp, histResp] = await Promise.all([
      fetch('/status'), fetch('/trades'), fetch('/history')
    ]);
    statusData = await statusResp.json();
    const trades = await tradesResp.json();
    historyData = await histResp.json();

    $('#cycle-text').textContent = 'Cycle ' + statusData.cycle_count;
    $('#last-update').textContent = 'Updated ' + new Date().toLocaleTimeString();

    renderOverview(statusData);
    renderPrices(statusData.prices || {});
    renderStrategies();
    renderTrades(trades);
    renderPositions();
    renderChart();
    renderStrategyCharts();
    renderReportSummary();
    renderReportTable();
  } catch(e) {
    $('#cycle-text').textContent = 'Connection error';
    $('#status-dot .dot').className = 'dot stale';
  }
  countdown = REFRESH_INTERVAL;
}

setInterval(() => {
  countdown--;
  const pct = ((REFRESH_INTERVAL - countdown) / REFRESH_INTERVAL) * 100;
  $('#refresh-fill').style.width = pct + '%';
  if (countdown <= 0) refresh();
}, 1000);

refresh();
</script>
</body>
</html>`
