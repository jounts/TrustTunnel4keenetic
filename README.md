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

При включённом Smart Routing в TUN-режиме трафик маршрутизируется автоматически:

```
[Пакет] → iptables mangle → TT_SMART chain
  ├─ dst в tt_tunnel (DNS-resolved)?  → через туннель
  ├─ dst в tt_domestic (GeoIP CIDR)?  → напрямую через ISP
  └─ остальное                        → через туннель (безопасно по умолчанию)
```

- **tt_domestic** — ipset с CIDR-блоками домашней страны (загружается из [country-ip-blocks](https://github.com/herrbischoff/country-ip-blocks))
- **tt_tunnel** — ipset, наполняемый dnsmasq при резолве доменов из пользовательского списка (решает проблему CDN)
- **dnsmasq** — запускается на отдельном порту (5354), не конфликтует с DNS роутера

## Установка

### Требования

- OPKG Entware — установлен на роутере
    - для [KN-1012](https://support.keenetic.com/hero/kn-1012/en/18481-opkg.html)
    - для [NC-1012](https://support.netcraze.ru/giga/nc-1012/ru/18481-opkg.html)
- `curl` — `opkg install curl`

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

## Удаление

### Через скрипт (рекомендуется)

```bash
curl -fsSL https://raw.githubusercontent.com/jounts/TrustTunnel4keenetic/master/scripts/uninstall.sh | sh
```

### Локально (если уже установлен)

```bash
/opt/trusttunnel_client/uninstall.sh
```

Скрипт останавливает сервисы, удаляет init-скрипты, NDM-хуки и директорию `/opt/trusttunnel_client`.

## Настройка

### Интерактивная

```bash
/opt/trusttunnel_client/configure.sh
```

### Ручная

1. Конфигурация клиента: `/opt/trusttunnel_client/trusttunnel_client.toml`
2. Режим работы: `/opt/trusttunnel_client/mode.conf`
3. Веб-панель: `/opt/trusttunnel_client/manager.conf`

### Аутентификация веб-панели

По умолчанию используется NDM-аутентификация — вход через учётные записи роутера Keenetic. Для отключения аутентификации:

```bash
vi /opt/trusttunnel_client/manager.conf
```

```
LISTEN_ADDR=":8080"
AUTH_MODE="ndm"
```

| Значение `AUTH_MODE` | Описание |
|----------------------|----------|
| `ndm` (по умолчанию) | Аутентификация через учётные записи Keenetic (challenge-response) |
| `none` | Аутентификация отключена |

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

### Аутентификация (публичные)

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/api/auth/login` | Вход через учётную запись Keenetic |
| `POST` | `/api/auth/logout` | Завершение сессии |
| `GET` | `/api/auth/check` | Проверка статуса аутентификации |

### Сервис

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/api/status` | Статус сервиса (running, PID, uptime, mode, health check) |
| `POST` | `/api/service/{action}` | Управление сервисом (`start`, `stop`, `restart`, `reload`) |
| `GET` | `/api/system` | Информация о системе (модель, прошивка, NDMS версия, FW backend) |

### Конфигурация

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/api/config` | Чтение конфигурации (TOML + mode.conf) |
| `PUT` | `/api/config` | Запись конфигурации |
| `GET` | `/api/mode` | Текущий режим |
| `PUT` | `/api/mode` | Смена режима (SOCKS5/TUN) |

### Логи

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/api/logs?lines=100&source=client` | Последние N строк лога (`source`: `client` или `manager`) |
| `GET` | `/api/logs/stream?source=client` | SSE-поток логов (live) |
| `DELETE` | `/api/logs` | Очистка лог-файлов |

### Обновления

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/api/update/check` | Проверка обновлений (клиент + менеджер) |
| `POST` | `/api/update/install` | Установка обновления клиента |
| `POST` | `/api/update/install-manager` | Установка обновления менеджера (self-update) |

### Smart Routing

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/api/routing` | Конфигурация и статистика Smart Routing |
| `PUT` | `/api/routing` | Обновление настроек Smart Routing |
| `GET` | `/api/routing/domains` | Список доменов для туннеля |
| `PUT` | `/api/routing/domains` | Обновление списка доменов |
| `POST` | `/api/routing/update-nets` | Обновление GeoIP-списков |

Все эндпоинты кроме `/api/auth/*` требуют аутентификации (сессионный cookie). Режим аутентификации настраивается в `manager.conf` (`AUTH_MODE`).

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
│   ├── routing/                # Smart routing manager
│   ├── service/                # Process manager, config, updater
│   ├── ndm/                    # RCI API клиент (NDMS 4/5 compat)
│   └── platform/               # Системная информация (NDMS version, FW backend)
├── web/                        # Vue 3 + Vite + Tailwind CSS
├── scripts/
│   ├── hooks/                  # NDM хуки
│   ├── init.d/                 # Init-скрипты
│   ├── ndms-compat.sh          # Слой совместимости NDMS 4/5 (iptables/nftables)
│   ├── smart-routing.sh        # Smart Routing (ipset, dnsmasq, policy routing)
│   ├── install.sh              # Установщик
│   ├── uninstall.sh            # Удаление
│   └── configure.sh            # Интерактивная настройка
├── packaging/                  # Сборка .ipk
├── .github/workflows/          # CI/CD
├── Makefile
└── go.mod
```

## Smart Routing (GeoIP-маршрутизация)

Позволяет автоматически направлять трафик к ресурсам в домашней стране напрямую, а весь остальной трафик — через туннель. Работает только в **TUN-режиме**.

### Принцип работы

1. При старте загружаются CIDR-блоки домашней страны в ipset `tt_domestic`
2. dnsmasq запускается на отдельном порту и наполняет ipset `tt_tunnel` IP-адресами доменов из пользовательского списка
3. iptables mangle-правила маркируют пакеты к домашним IP, направляя их мимо туннеля
4. Домены из списка `tt_tunnel` переопределяют domestic-правила (решает проблему CDN)

### Включение

1. Откройте веб-панель → **Маршрутизация**
2. Включите Smart Routing, выберите домашнюю страну
3. В настройках DNS роутера (Keenetic → Интернет-фильтры → DNS) добавьте сервер `<IP роутера>:5354`
4. Опционально: добавьте домены, которые всегда должны идти через туннель (YouTube, Netflix и т.д.)

### Конфигурация (`mode.conf`)

```
SR_ENABLED="yes"
SR_HOME_COUNTRY="RU"
SR_DNS_PORT="5354"
SR_DNS_UPSTREAM="1.1.1.1"
```

### Файлы на роутере

| Файл | Описание |
|------|----------|
| `/opt/trusttunnel_client/routing/domains.txt` | Домены, принудительно направляемые через туннель |
| `/opt/trusttunnel_client/routing/domestic_nets.txt` | CIDR-блоки домашней страны (автозагрузка) |
| `/opt/etc/dnsmasq.d/trusttunnel.conf` | Генерируемый конфиг dnsmasq |

### Зависимости

Smart Routing использует пакеты `dnsmasq-full` и `ipset` из Entware. Они устанавливаются автоматически при первом включении или вручную:

```bash
opkg update && opkg install dnsmasq-full ipset
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

## Благодарности

- [TrustTunnel VPN](https://github.com/TrustTunnel/TrustTunnelClient) — VPN-клиент, для которого создан этот менеджер
- [TrustTunnel-Keenetic](https://github.com/artemevsevev/TrustTunnel-Keenetic) — проект, вдохновивший на создание этой обёртки

## Лицензия

MIT
