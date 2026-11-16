# Matchingo — Go Order Matching Engine

A personal fork of [GOnevo/matchingo](https://github.com/GOnevo/matchingo), with regression-tested fixes for order validation, quote accounting, stop/OCO lifecycle, and queue handling. **Go 语言订单撮合引擎：侧重输入校验、余额计算与边界行为修复。**

## 使用导航

[项目定位](#项目定位) · [优化说明](#优化说明) · [本地运行](#本地运行) · [验证与边界](#验证与边界) · [原项目文档](#原项目文档)

## 项目定位

适合学习、集成和验证内存订单簿、限价/市价撮合、止损限价与 OCO 订单逻辑。本仓库是社区贡献 fork，不是原作者的官方发行版，也不是包含账户余额、清结算和持久化的完整交易所系统。

优化代码位于 `improve/reliability-performance-20261116`，独立上游贡献位于 `fix/reliability-review`。原有 Go module/import 路径保持 `github.com/gonevo/matchingo`；直接 `go get` 原项目不会自动取得本 fork 尚未合并的补丁。

## 优化说明

| 场景 | 修改后的行为 | 对使用者的意义 |
| --- | --- | --- |
| nil、无效方向、已取消订单、重复活跃 ID | 在修改订单簿前返回错误 | 避免错误输入导致崩溃或意外撮合 |
| 报价预算不能整除成交价格 | 保留未花费余额，并报告剩余量 | 固定精度截断不再直接吞掉剩余预算 |
| 市价单没有可成交对手盘 | 报告未成交和取消状态 | 调用方能正确处理剩余量 |
| 止损触发、OCO 撤单 | 清理索引、保留独立订单对象、只从所在订单簿撤单 | 修复订单生命周期不一致 |
| 重复撤单、不存在的订单、越界队列索引 | 返回失败，不修改数量、深度和总量 | 边界调用更可预测 |
| 队列转 slice 与查询深度 | 预分配结果，移除意外标准输出 | 减少分配，避免库函数污染应用日志 |

对于按报价币种控制预算的市价买单，请使用 `NewMarketQuoteOrder`。`NewMarketOrder` 的数量按基础币种处理。不要把报价预算与基础币数量混用；以 `Done` 的实际成交与剩余量处理后续业务。

## 本地运行

需要 Go 1.20 或以上；本 fork 在 Go 1.25.11 上验证。

```sh
git clone --branch improve/reliability-performance-20261116 https://github.com/nemo-crypto/matchingo.git
cd matchingo
go test ./...
go run ./example
```

若已克隆，先确认当前分支及工作区，再更新相应分支。应用项目试用本地 fork 可使用 Go module 的本地替换，例如：

```sh
go mod edit -replace github.com/gonevo/matchingo=/absolute/path/to/matchingo
```

替换路径应指向你的实际 checkout；不应将个人机器的绝对路径直接提交给团队。

## 验证与边界

```sh
go test -race ./... -count=1
go vet ./...
go test ./tests -run '^$' -bench BenchmarkQueueSliceReview -benchmem
```

已通过全套 race 测试和 vet。1,000 条订单队列微基准的中位耗时为 39,066 → 17,388 ns/op，分配为 27 → 16 次；这只是该局部操作的结果，不代表整套交易系统吞吐量。

订单簿是可变内存结构，调用方仍需串行化操作。余额校验、持久化、精度溢出与结算舍入、生产资金安全需要另外验证。详见 [逐项审查报告](docs/reliability-performance-review.md)。

## 贡献与来源

[本次上游 PR #6](https://github.com/GOnevo/matchingo/pull/6)；之前的止损/OCO 修复见 [#3](https://github.com/GOnevo/matchingo/pull/3)、[#4](https://github.com/GOnevo/matchingo/pull/4)、[#5](https://github.com/GOnevo/matchingo/pull/5)。

修复与回归测试会向原项目提交 PR；提交不等于上游已接受。保留原项目署名和许可证，见 [LICENSE](LICENSE.md)。本 fork 的 [PR 列表](https://github.com/nemo-crypto/matchingo/pulls) 记录集成情况。

## 原项目文档

以下保留原项目的 API 介绍；其原始安装命令指向上游，使用本 fork 请参考上方步骤。

[![Go Report Card](https://goreportcard.com/badge/github.com/gonevo/matchingo)](https://goreportcard.com/report/github.com/gonevo/matchingo)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/gonevo/matchingo)
![GitHub](https://img.shields.io/github/license/gonevo/matchingo)

# Matchingo

Incredibly fast matching engine for HFT written in Golang

### Features

- supports **MARKET**, **LIMIT**, **STOP-LIMIT**, **OCO** order types
- supports _time-in-force_ (**GTK**, **FOK**, **IOC**) parameters for **LIMIT** orders
- does not use [shopspring/decimal](https://github.com/shopspring/decimal) for higher performance
- uses [lite decimal](https://github.com/nikolaydubina/fpdecimal) for price and quantity arguments
- well tested code

##### Order quantity

> **It is very important!** Each SYMBOL is combination of BASE and QUOTE currencies.
> For example, **BTC/USD** where **BTC** is **BASE** currency, **USD** is **QUOTE** currency.

Each order **MUST HAVE** _quantity_ parameter but **pay attention**:
for **MARKET BUY** orders you have pass **QUOTE quantity**.

Examples for **BTC/USD** symbol:

- for **MARKET** **SELL** order 10 BTC, you have to pass quantity=10
- for **MARKET** **BUY** order 10 BTC, you have to pass maximum quantity of USD for attempt to buy 10 BTC

> This may seem inconvenient, but it is a precautionary measure for **double spending**: when a user places a
> **MARKET** **BUY** in asynchronous way, you will be able to freeze the correct quantity on his balance.

### Installation

```shell
go get github.com/gonevo/matchingo
```

### Usage

### Decimal
Matchingo works with fpdecimal values, so feel free to use:

- `matchingo.FromInt(number int)`
- `matchingo.FromFloat(number float64)`

#### Order instance initialization parameters

- `matchingo.NewMarketOrder(orderID string, side Side, quantity fpdecimal.Decimal)`
- `matchingo.NewLimitOrder(orderID string, side Side, quantity, price fpdecimal.Decimal, tif TIF, oco string)`
- `matchingo.NewStopOrder(orderID string, side Side, quantity, price, stop fpdecimal.Decimal, oco string)`

> oco parameter is ID of another order from **OCO** orders set

#### Order processing

- `matchingo.Process(order *Order) (done *Done, err Error)`

#### Done instance

**Process()** returns **Done** instance which contains:

- **Order**: processed order
- **Trades**: slice of orders from the order book which are participants for this trade
    - **OrderID**: participant-order ID
    - **Price**: concrete trade price, string
    - **Quantity**: concrete trade quantity, string
    - **Role**: **TAKER** or **MAKER**, string
    - **IsQuote**: _true_ for **QUOTE quantity** orders
- **Canceled**: slice of order IDs which was cancelled for this processing (**IOC**, **OCO**), can be empty
- **Activated**: slice of order IDs which was activated for this processing (**STOP** orders), can be empty
- **Left**: _fpdecimal.Decimal_ value of left quantity for this processing, can be _fpdecimal.Zero_
- **Processed**: _fpdecimal.Decimal_ value of processed quantity for this processing, can be _fpdecimal.Zero_
- **Stored**: boolean, _true_ if order or its part was appended to **stop book** or **order book**

For example:

```json
{
  "order": {
    "orderID": "order2",
    "role": "TAKER",
    "price": "10.00000",
    "isQuote": false,
    "quantity": "9.00000"
  },
  "trades": [
    {
      "orderID": "order2",
      "role": "TAKER",
      "price": "10.00000",
      "isQuote": false,
      "quantity": "9.00000"
    },
    {
      "orderID": "order1",
      "role": "MAKER",
      "price": "10.00000",
      "isQuote": false,
      "quantity": "9.00000"
    }
  ],
  "canceled": [],
  "activated": [],
  "left": "0",
  "processed": "9.00000",
  "stored": false
}
```

### Search
You can search your order at any time using ID

- `matchingo.GetOrder(id string) *Order`

> it returns an instance of Order or nil if Order not found

### Canceling
You can cancel your order at any time

- `matchingo.CancelOrder(id string) *Order`

> it returns an instance of Order if it was canceled or nil if Order not found

### Depth
You can see orderbook depth at any time

- `matchingo.Depth() map[string]string`
- `matchingo.DepthJSON() string`

For example:
```json
{
   "ask": {
      "21.00000": "20.00000",
      "22.00000": "10.00000",
      "23.00000": "20.00000"
   },
   "bid": {
      "10.00000": "20.00000",
      "11.00000": "10.00000",
      "12.00000": "20.00000"
   }
}
```

> where key is a price, value is a volume 

### Example

```golang
package main

import (
	"fmt"

	"github.com/gonevo/matchingo"
)

func main() {
	orderBook := matchingo.NewOrderBook()
	matchingo.SetDecimalFraction(5) // default is 3

	done1, _ := orderBook.Process(matchingo.NewLimitOrder("order1", matchingo.Sell, matchingo.FromInt(10), matchingo.FromInt(10), "", ""))
	fmt.Println(done1)
	fmt.Println(orderBook)

	done2, _ := orderBook.Process(matchingo.NewLimitOrder("order2", matchingo.Buy, matchingo.FromInt(5), matchingo.FromInt(10), "", ""))
	fmt.Println(done2)
	fmt.Println(orderBook)

	done3, _ := orderBook.Process(matchingo.NewMarketOrder("order3", matchingo.Buy, matchingo.FromFloat(5)))
	fmt.Println(done3)
	fmt.Println(orderBook)
}
```

### Benchmark

```shell
goos: linux
goarch: amd64
cpu: Intel(R) Core(TM) i7-8565U CPU @ 1.80GHz
BenchmarkLimitOrders-8 

Transactions per second (avg): 1139342.359570
 1000000              1755 ns/op            1011 B/op         14 allocs/op

```

### License

**matchingo** is open-sourced software licensed under the [MIT license](./LICENSE.md).

[Vano Devium](https://github.com/vanodevium/)

---

Made with ❤️ in Ukraine

[!["Buy Me A Coffee"](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/vanodevium)

### Fork reliability review

See [the review and validation report](docs/reliability-performance-review.md) for input validation, quote-budget handling, regression tests, and measured allocation improvements.
