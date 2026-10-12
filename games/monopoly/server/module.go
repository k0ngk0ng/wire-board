package monopoly

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Action struct {
	Type     string      `json:"type"`
	Window   int         `json:"window"`
	Property int         `json:"property,omitempty"`
	Amount   int         `json:"amount,omitempty"`
	Choice   string      `json:"choice,omitempty"`
	Offer    *TradeTerms `json:"offer,omitempty"`
	TradeID  int         `json:"tradeId,omitempty"`
}

func strict(raw json.RawMessage, value any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}

func Module() gameapi.Module {
	return gameapi.Module{
		ID: "monopoly", Name: "强手棋", MinPlayers: 2, MaxPlayers: 6,
		Configure: func(n int, raw json.RawMessage) (json.RawMessage, error) {
			if n < 2 || n > 6 {
				return nil, errors.New("强手棋需要 2～6 人")
			}
			o := Options{AuctionSeconds: 20, TurnSeconds: 120, TradeSeconds: 90}
			if len(raw) > 0 {
				if err := strict(raw, &o); err != nil {
					return nil, errors.New("无效强手棋设置")
				}
			}
			if o.AuctionSeconds < 5 || o.AuctionSeconds > 60 || o.TurnSeconds < 30 || o.TurnSeconds > 300 || o.TradeSeconds < 15 || o.TradeSeconds > 180 {
				return nil, errors.New("计时设置超出范围")
			}
			return json.Marshal(o)
		},
		Start: func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
			var o Options
			if err := strict(raw, &o); err != nil {
				return gameapi.Match{}, err
			}
			if ctx.Players < 2 || ctx.Players > 6 {
				return gameapi.Match{}, errors.New("人数不符合要求")
			}
			s, err := newState(ctx.Players, o, ctx.Now)
			if err != nil {
				return gameapi.Match{}, err
			}
			return s.Envelope()
		},
		Apply: apply,
		View: func(match gameapi.Match, viewer int) (any, error) {
			s, err := decode(match)
			if err != nil {
				return nil, err
			}
			return map[string]any{"players": s.Players, "properties": s.Properties, "board": board, "turn": s.Turn, "phase": s.Phase, "window": s.Window, "dice": s.Dice, "rollSerial": s.RollSerial, "utilityDice": s.UtilityDice, "doubles": s.Doubles, "houses": s.Houses, "hotels": s.Hotels, "auction": s.Auction, "debt": s.Debt, "transfer": s.Transfer, "eliminated": s.Eliminated, "controls": s.controls(viewer), "lastCard": s.LastCard, "cards": cards, "offers": s.tradeViews(viewer), "auctionActors": s.auctionActors()}, nil
		},
		AcceptStale: func(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) bool {
			s, err := decode(match)
			if err != nil {
				return false
			}
			var a Action
			if strict(raw, &a) != nil {
				return false
			}
			if a.Type == "trade_accept" || a.Type == "trade_reject" || a.Type == "trade_cancel" {
				if i := s.tradeIndex(a.TradeID); i >= 0 {
					t := s.Trades[i]
					return ctx.Now.UnixMilli() < t.Expires && ((a.Type == "trade_cancel" && t.From == ctx.Actor) || (a.Type != "trade_cancel" && t.To == ctx.Actor))
				}
			}
			return s.Phase == "auction" && a.Window == s.Window && a.Type == "bid" && ctx.Now.UnixMilli() < s.Auction.Expires
		},
		OnTimer: func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
			s, err := decode(match)
			if err != nil {
				return match, err
			}
			if ctx.Actor != -1 || match.SystemDeadline == 0 || ctx.Now.UnixMilli() < match.SystemDeadline {
				return match, errors.New("系统事件尚未到期")
			}
			s.Trades = slices.DeleteFunc(s.Trades, func(t Trade) bool { return t.Expires <= ctx.Now.UnixMilli() })
			if s.Auction != nil && ctx.Now.UnixMilli() >= s.Auction.Expires {
				if err = s.finishAuction(ctx.Now); err != nil {
					return match, err
				}
			}
			return s.Envelope()
		},
		BotAction: botAction,
		Reclaim: func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
			s, err := decode(match)
			if err != nil {
				return match, err
			}
			if s.dueActor() != ctx.Actor {
				return match, errors.New("无需恢复当前玩家的操作时间")
			}
			s.Deadline = ctx.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
			return s.Envelope()
		},
	}
}

func apply(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, err := decode(match)
	if err != nil {
		return match, err
	}
	var a Action
	if err = strict(raw, &a); err != nil {
		return match, errors.New("无效游戏行动")
	}
	tradeResponse := a.Type == "trade_accept" || a.Type == "trade_reject" || a.Type == "trade_cancel"
	if (!tradeResponse && a.Window != s.Window) || !slices.Contains(s.actors(), ctx.Actor) {
		return match, errors.New("操作窗口已结束或当前不能行动")
	}
	if s.Phase == "auction" && ctx.Now.UnixMilli() >= s.Auction.Expires {
		return match, errors.New("拍卖已经截止")
	}
	p := ctx.Actor
	switch a.Type {
	case "build":
		err = s.build(p, a.Property, ctx.Now)
	case "sell_building":
		err = s.sellBuilding(p, a.Property, ctx.Now)
	case "sell_group":
		err = s.sellGroup(p, a.Property, ctx.Now)
	case "trade_offer":
		err = s.offerTrade(p, a.Offer, ctx.Now)
	case "trade_accept", "trade_reject", "trade_cancel":
		err = s.answerTrade(p, a.TradeID, a.Type, ctx.Now)
	case "roll":
		err = s.roll(p, ctx.Now)
	case "jail_pay":
		err = s.leaveJail(p, "", false, ctx.Now)
	case "jail_card":
		err = s.leaveJail(p, a.Choice, true, ctx.Now)
	case "buy":
		if s.Phase != "purchase" || p != s.Turn {
			return match, errors.New("当前不能买地")
		}
		id := s.Players[p].Position
		if board[id].Price <= 0 || s.Properties[id].Owner != -1 || s.available(p) < board[id].Price {
			return match, errors.New("无法购买该地产")
		}
		s.pay(p, -1, board[id].Price)
		s.Properties[id].Owner = p
		s.note("玩家 %d 以 %d 购买了%s", p+1, board[id].Price, board[id].Name)
		err = s.drain(ctx.Now)
	case "auction":
		if s.Phase != "purchase" || p != s.Turn {
			return match, errors.New("当前不能发起拍卖")
		}
		err = s.beginAuction(s.Players[p].Position, ctx.Now)
	case "bid":
		err = s.bid(p, a.Amount, a.Property, ctx.Now)
	case "auction_pass":
		if s.Phase != "auction" {
			return match, errors.New("当前没有拍卖")
		}
		s.Auction.Withdrawn[p] = true
		if len(s.auctionActors()) == 0 {
			err = s.finishAuction(ctx.Now)
		}
	case "mortgage":
		err = s.mortgage(p, a.Property, ctx.Now)
	case "unmortgage":
		err = s.unmortgage(p, a.Property)
	case "bankrupt":
		err = s.bankrupt(p, ctx.Now)
	case "accept_mortgage":
		err = s.acceptMortgage(p, a.Choice, ctx.Now)
	case "end":
		if s.Phase != "end" || p != s.Turn {
			return match, errors.New("当前不能结束回合")
		}
		s.nextTurn()
		s.enter("roll", ctx.Now)
	case "wait":
		if p == s.dueActor() || slices.Contains(s.auctionActors(), p) || s.hasIncomingTrade(p) {
			return match, errors.New("仍有待处理操作")
		}
		s.Waiting[p] = true
	default:
		return match, errors.New("该行动尚不可用")
	}
	if err != nil {
		return match, err
	}
	// Asset management can fund bids, but the promised winning build must remain
	// executable. Reject the entire detached candidate on any violation.
	if s.buildingAuction() && s.Auction.Bidder >= 0 && s.buildKind(s.Auction.Bidder, s.Auction.Property) != s.Auction.Kind {
		return match, errors.New("操作会破坏当前最高报价的建设条件")
	}
	return s.Envelope()
}
