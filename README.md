# TrustTunnel Manager for Keenetic

Управляющая обёртка для [TrustTunnel VPN](https://github.com/TrustTunnel/TrustTunnelClient) на роутерах Keenetic / Netcraze. Веб-панель для управления, мониторинга и обновления клиента прямо из браузера.

## Возможности

- **Веб-панель управления** — Dashboard, настройки, маршрутизация, логи, обновления
- **Два режима работы** — SOCKS5 (Proxy) и TUN (полный перехват трафика)
- **Smart Routing (GeoIP)** — домашний трафик напрямую, зарубежный через туннель
- **NDM-хуки** — автостарт при WAN up, управление по расписанию, toggle по кнопке FN
- **Watchdog + Health Check** — автоматический перезапуск при сбоях
- **Обновление клиента** — проверка и установка обновлений через GitHub Releases
- **Поддержка NDMS 4 и NDMS 5** — автоопределение версии, совместимость iptables/nftables
- **Кросс-платформенность** — mipsel, mips, aarch64, armv7

## Совместимость с NDMS

| Компонент | NDMS 4 | NDMS 5 |
|-----------|--------|--------|
| Firewall | iptables (legacy) | iptables / nftables (авто) |
| Интерфейсы | `nwg{N}` → `tun{N}` | Автоопределение префикса |
| ndmc CLI | Стандартный синтаксис | С обработкой ошибок совместимости |
| ipset | ipset (hash:net) | ipset или nft sets |
| Smart Routing | dnsmasq + ipset | dnsmasq + ipset/nftset |

Определение версии NDMS выполняется автоматически при старте. Информация о версии отображается в веб-панели (Dashboard → Система, Маршрутизация → Статистика).

## Архитектура

```
[Браузер] → [trusttunnel-manager :8080] → [S99trusttunnel] → [trusttunnel_client]
                    ↕                              ↕
              [RCI API :79]                  [NDM хуки]
              (ndmc команды)           (wan.d, netfilter.d, ...)
                                               ↕
                                       [ndms-compat.sh]
                                    (iptables / nftables)
```

### Smart Routing

```
[Клиент] → [iptables/nft mangle] → tt_tunnel ipset? → Через туннель (default route)
                                  → tt_domestic ipset? → MARK 0x100 → Таблица 100 → ISP gateway
                                  → Остальное → Через туннель

[dnsmasq :5354] → Разрешает домены из domains.txt → Добавляет IP в tt_tunnel ipset
```

- **tt_domestic** — CIDR-диапазоны домашней страны (из github.com/herrbischoff/country-ip-blocks)
- **tt_tunnel** — IP, разрешённые dnsmasq для доменов, которые должны идти через туннель
- Приоритет: `tt_tunnel` > `tt_domestic` > всё остальное через туннель

`trusttunnel-manager` — Go-бинарник со встроенной Vue 3 SPA. Управляет клиентом через init-скрипты, взаимодействует с NDM через RCI API.

## Установка

### Быстрая установка (curl)

```bash
curl -fsSL https://raw.githubusercontent.com/jounts/TrustTunnel4keenetic/master/scripts/install.sh | sh
```

### Установка через OPKG

```bash
# Скачайте .ipk для вашей архитектуры
wget https://github.com/jounts/TrustTunnel4keenetic/releases/latest/download/trusttunnel-manager_<version>_<arch>.ipk

# Установите
opkg install trusttunnel-manager_*.ipk
```

### Зависимости для Smart Routing (опционально)

```bash
opkg install dnsmasq-full ipset
```

### Определение архитектуры

| Архитектура | Модели |
|-------------|--------|
| `mipsel` | Большинство Keenetic (KN-1010, KN-1810 и др.) |
| `mips` | Старые модели |
| `aarch64` | Keenetic Peak, Ultra (KN-1811, KN-1810 v2) |
| `armv7` | Модели с ARM CPU |

```bash
uname -m  # покажет архитектуру
```

## Настройка

### Интерактивная

```bash
/opt/trusttunnel_client/configure.sh
```

### Ручная

1. Конфигурация клиента: `/opt/trusttunnel_client/trusttunnel_client.toml`
2. Режим работы: `/opt/trusttunnel_client/mode.conf`
3. Веб-панель: `/opt/trusttunnel_client/manager.conf`

### Пароль для веб-панели

```bash
vi /opt/trusttunnel_client/manager.conf
```

```
LISTEN_ADDR=":8080"
USERNAME="admin"
PASSWORD="your_password"
```

### Smart Routing

Доступен только в режиме **TUN**. Настраивается через веб-панель (Маршрутизация) или `mode.conf`:

```bash
SR_ENABLED="yes"
SR_HOME_COUNTRY="RU"
SR_DNS_PORT="5354"
SR_DNS_UPSTREAM="1.1.1.1"
```

Домены для принудительной маршрутизации через туннель: `/opt/trusttunnel_client/routing/domains.txt`

## Запуск

```bash
# Запуск веб-панели
/opt/etc/init.d/S98trusttunnel-manager start

# Запуск VPN клиента
/opt/etc/init.d/S99trusttunnel start
```

Веб-панель: `http://<IP-роутера>:8080`

## REST API

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/api/status` | Статус сервиса (running, PID, uptime, mode, health check) |
| `POST` | `/api/service/{start,stop,restart}` | Управление сервисом |
| `GET` | `/api/config` | Чтение конфигурации (TOML + mode.conf) |
| `PUT` | `/api/config` | Запись конфигурации |
| `GET` | `/api/mode` | Текущий режим |
| `PUT` | `/api/mode` | Смена режима (SOCKS5/TUN) |
| `GET` | `/api/logs?lines=100` | Последние N строк лога |
| `GET` | `/api/logs/stream` | SSE-поток логов (live) |
| `GET` | `/api/update/check` | Проверка обновлений |
| `POST` | `/api/update/install` | Установка обновления клиента |
| `GET` | `/api/system` | Информация о системе (модель, прошивка, NDMS версия, FW backend) |
| `GET` | `/api/routing` | Конфигурация и статистика Smart Routing |
| `PUT` | `/api/routing` | Обновление настроек Smart Routing |
| `GET` | `/api/routing/domains` | Список доменов для туннеля |
| `PUT` | `/api/routing/domains` | Обновление списка доменов |
| `POST` | `/api/routing/update-nets` | Обновление GeoIP-списков |

Аутентификация: HTTP Basic Auth (если задан пароль).

## NDM-хуки

| Хук | Файл | Описание |
|-----|------|----------|
| WAN up/down | `wan.d/010-trusttunnel.sh` | Автостарт при получении IP |
| Interface change | `iflayerchanged.d/trusttunnel.sh` | Перезапуск при падении интерфейса |
| Netfilter | `netfilter.d/trusttunnel.sh` | Восстановление firewall-правил (iptables/nftables) |
| Schedule | `schedule.d/trusttunnel.sh` | Start/stop по расписанию NDMS |
| Button | `button.d/trusttunnel.sh` | Toggle по кнопке FN |

## Структура проекта

```
TrustTunnel4keenetic/
├── cmd/trusttunnel-manager/    # Go точка входа
├── internal/
│   ├── api/                    # REST API handlers + middleware
│   ├── service/                # Process manager, config, updater
│   ├── ndm/                    # RCI API клиент (NDMS 4/5 compat)
│   ├── routing/                # Smart Routing manager
│   └── platform/               # Системная информация (NDMS version, FW backend)
├── web/                        # Vue 3 + Vite + Tailwind CSS
├── scripts/
│   ├── hooks/                  # NDM хуки
│   ├── init.d/                 # Init-скрипты
│   ├── ndms-compat.sh          # Слой совместимости NDMS 4/5 (iptables/nftables)
│   ├── smart-routing.sh        # Smart Routing (ipset, dnsmasq, policy routing)
│   ├── install.sh              # Установщик
│   └── configure.sh            # Интерактивная настройка
├── packaging/                  # Сборка .ipk
├── .github/workflows/          # CI/CD
├── Makefile
└── go.mod
```

## Сборка из исходников

### Требования

- Go >= 1.21
- Node.js >= 20
- npm

### Локальная сборка

```bash
# Сборка Vue + Go для хост-системы
make build

# Кросс-компиляция для всех архитектур
make build-all

# Сборка .ipk пакетов
make ipk
```

### Разработка

```bash
# Установка зависимостей фронтенда
cd web && npm install && cd ..

# Сборка Vue
cd web && npm run build && cd ..

# Запуск Go (dev)
go run ./cmd/trusttunnel-manager -dev
```

## Лицензия

MIT
