# squadup — план разработки

Oct 23, 2026 · @Alex

squadup — CLI для Claude Code: хранит пресеты команды агентов (агенты, модели, права), переключает их одной командой и всегда умеет вернуть исходный конфиг (Initial). Название утверждено: на 23.10.2026 имя `squadup` свободно, а `loadout` и `claude-loadout` заняты.

## Границы v1

v1 управляет только тем, что составляет «команду»: субагентами и настройками, которые на них влияют. Всё прочее в `~/.claude` тула не трогает.

**Входит**

- Пресет: набор субагентов, главный агент (опционально), модель по умолчанию, effort и права (`permissions`).
- Initial: снимок исходного конфига при первом запуске и возврат к нему одной командой.
- Два способа включить пресет: `squad use` пишет в `~/.claude`, `squad run` запускает `claude` с пресетом и ничего не пишет.
- Интерактивный TUI плюс те же действия обычными командами для скриптов.
- Экспорт и импорт пресета одним файлом.
- Список моделей, реально доступных аккаунту.
- macOS и Linux официально; Windows по принципу «должно работать» (пути через `path`, без симлинков).

**Не входит в v1** (кандидаты в v1.1+)

- Создание скиллов: в агенте можно сослаться на скилл по имени в поле `skills`, но сами `SKILL.md` тула не пишет.
- Хуки, MCP-серверы, `CLAUDE.md`, `env`, `statusLine` в `settings.json` (исключение — опциональная статус-строка squadup, только по явной команде).
- Проектный scope (`.claude/agents` внутри репозитория).
- Облачный шэринг, аккаунты, мониторинг токенов в живой сессии.

## Ключевые решения

Тула пишется на Go и поставляется одним исполняемым файлом без зависимостей: рекомендуемая установка Claude Code тоже нативная, и Node.js у пользователя может не быть. Интерфейс — на стеке Charm.

| Вопрос | Решение | Почему |
| --- | --- | --- |
| Язык | Go, актуальная стабильная версия | Один бинарь под каждую ОС, мгновенный запуск, никаких рантаймов у пользователя |
| Команды и флаги | `spf13/cobra` | Стандарт для Go-CLI: сабкоманды, help, автодополнение для zsh/bash/fish |
| TUI | `charmbracelet/bubbletea` + `bubbles` + `lipgloss` | Логика экранов, готовые списки/таблицы/спиннеры и стили в духе CSS |
| Формы | `charmbracelet/huh` | Редактор агента и мастера без ручной вёрстки полей |
| YAML | `goccy/go-yaml` (исполнитель сверяет статус библиотеки) | Frontmatter агентов и `preset.yaml` |
| Правка settings.json | `tidwall/gjson` + `tidwall/sjson` | Меняет ключи прямо в тексте JSON: порядок и форматирование чужих ключей сохраняются (стандартный `encoding/json` порядок теряет) |
| Валидация | Go-структуры + свои функции `Validate()` | Одна схема для файлов, импорта, линтера и UI |
| Тесты | `go test`, `rogpeppe/go-internal/testscript` (e2e CLI), `teatest` из Charm (снэпшоты TUI) | testscript пишет сценарии как команды терминала с проверкой файлов |
| Линтер кода | `golangci-lint` | Стандарт |
| Релизы | GoReleaser | Один конфиг: бинарники под все ОС, GitHub Release, Homebrew tap, чексуммы |
| GIF для README | `charmbracelet/vhs` | Запись терминала по сценарию, воспроизводимо |
| Данные тулы | `~/.squadup/` (переопределяется `SQUADUP_HOME`) | Отдельно от `~/.claude`, легко чистить и бэкапить |
| Применение пресета | Копирование файлов + манифест с хэшами, без симлинков | Работает на Windows, даёт детекцию ручных правок |
| settings.json | Тула владеет только ключами `model`, `effortLevel`, `agent`, `permissions` (плюс `statusLine` по явному включению) | Хуки, env и плагины пользователя никогда не задеваются |
| Шаблоны | Вшиваются в бинарь через `embed` | Работают офлайн, ничего не скачивается |
| Язык интерфейса | Только английский | Требование автора |
| Лицензия | MIT | Стандарт для dev-тулов |

## Опора на Claude Code

Всё ниже сверено с официальной документацией 23.10.2026; исполнитель перепроверяет флаги через `claude --help` на своей машине, потому что Claude Code обновляется часто.

| Что | Факт | Как используем |
| --- | --- | --- |
| Конфиг-папка | `~/.claude`, а если задан `CLAUDE_CONFIG_DIR` — она; на Windows `%USERPROFILE%\.claude` | Путь всегда резолвится через эту переменную; флаг `--claude-dir` переопределяет |
| Пользовательские агенты | `~/.claude/agents/**/*.md`: YAML frontmatter + тело (системный промпт); обязательны `name` и `description`; подпапки допустимы, идентичность только по `name` | Сюда пишет `squad use`; сканируем рекурсивно |
| Горячая перезагрузка | Правки в `~/.claude/agents/` подхватываются за несколько секунд без рестарта; исключение — первый файл в новой папке `agents` | `use` работает при открытой сессии; если папки не было — советуем рестарт |
| Имя агента | До 256 символов, без `:`, не начинается с `-`; иначе файл молча пропускается; дубли имён — грузится один случайный | Валидируем строже: `^[a-z0-9][a-z0-9-]{0,63}$`, уникальность |
| Поля frontmatter | `tools`, `disallowedTools`, `model`, `permissionMode`, `maxTurns`, `skills`, `mcpServers`, `hooks`, `memory`, `background`, `effort`, `isolation`, `color`, `initialPrompt`, `omitClaudeMd`, `autoCompactWindow`; неизвестное поле молча игнорируется | Go-структура и валидация 1:1; линтер ловит опечатки (`disallowed_tools`, `Model`) |
| Значения полей | `model`: `sonnet`, `opus`, `haiku`, `fable`, полный ID или `inherit`; `permissionMode`: `default`, `acceptEdits`, `auto`, `dontAsk`, `bypassPermissions`, `plan`; `effort`: `low`…`max`; `color`: red, blue, green, yellow, purple, orange, pink, cyan; `memory`: user, project, local | Выбор из списков в UI, не свободный ввод |
| Временные агенты | `claude --agents '<json>'`: ключ = имя, `prompt` = тело, остальные поля как во frontmatter; `color` не принимается; приоритет выше `~/.claude/agents`; невалидный JSON — выход при старте | Основа `squad run` |
| Временные настройки | `claude --settings <файл или JSON>` перекрывает те же ключи settings.json на сессию, файл до 2 МиБ | `squad run` передаёт права и модель |
| Главный агент | `claude --agent <name>` или ключ `agent` в settings.json; промпт агента целиком заменяет системный промпт Claude Code | Поле `main` в пресете + явное предупреждение в UI |
| Модель и effort | Ключи `model` и `effortLevel`; приоритет: `/model` > `--model` > `ANTHROPIC_MODEL` > settings | Предупреждать, если в окружении задан `ANTHROPIC_MODEL` |
| Права | `permissions.allow`, `ask`, `deny`, `defaultMode`, `additionalDirectories`; порядок deny → ask → allow; синтаксис `Bash(npm run *)`, `Read(./.env)`, `WebFetch(domain:x.com)` | Редактор прав + готовые профили |
| Диагностика | `claude -v`; `claude auth status` (JSON, код 1, если не залогинен, поле `configDirectory`); `claude doctor` | `squad doctor` |

## Формат пресета

Пресет — это папка: `preset.yaml` с настройками плюс файлы агентов ровно в том виде, в каком их ждёт Claude Code. Так пресет можно тюнить руками в редакторе, а агента — скопировать без тулы.

```text
~/.squadup/
  config.json          # настройки тулы (тема, проба моделей)
  state.json           # активный пресет + манифест записанных файлов
  presets/
    initial/           # снимок исходного конфига, только чтение
      preset.yaml
      agents/*.md
    solo-indie/
      preset.yaml
      agents/
        planner.md
        reviewer.md
        tester.md
  backups/2026-10-23T22-10-05/   # копии перед каждой записью, храним 20 последних
  cache/models.json    # результат пробы моделей
  .lock                # защита от двух одновременных записей
```

`preset.yaml`:

```yaml
schema: 1
name: solo-indie            # slug: ^[a-z0-9][a-z0-9-]{0,63}$
title: Solo Indie
description: Planner, reviewer and tester for one-person apps
accent: cyan                # один из 8 цветов Claude Code
mode: replace               # replace | extend (агенты Initial остаются рядом)
main: null                  # имя агента для ключа agent или null
agents: [planner, reviewer, tester]   # порядок слотов в UI
settings:                   # только управляемые ключи; не задано = берём из Initial
  model: sonnet
  effortLevel: high
  permissions:
    defaultMode: acceptEdits
    allow: ["Bash(npm run *)"]
    ask: ["Bash(git push *)"]
    deny: ["Read(./.env)", "Read(./.env.*)"]
```

Файл агента (`agents/reviewer.md`) — чистый формат Claude Code, без служебных полей тулы:

```markdown
---
name: reviewer
description: Reviews diffs for bugs and risky changes. Use after any code change, before commit.
tools: Read, Grep, Glob, Bash
model: sonnet
effort: high
permissionMode: default
color: green
---
You are a strict code reviewer. Report findings ranked by severity...
```

Правила сериализации: порядок ключей frontmatter фиксирован (`name`, `description`, далее как в таблице документации), пустые поля не пишутся, тело сохраняется байт в байт. Чужие поля из импортированных файлов не теряются, а помечаются линтером.

## Initial и переключение

Главное обещание тулы: `squad use A` → `squad use B` → `squad restore` возвращает `~/.claude` в исходное состояние байт в байт. Всё остальное в этом разделе служит этому обещанию.

**Управляемые ключи settings.json** (`MANAGED_KEYS`): `model`, `effortLevel`, `agent`, `permissions`. Любой другой ключ тула читает, но никогда не пишет.

**Снимок Initial**

1. При первом запуске любой команды (кроме `--help` и `--version`) копируются все `~/.claude/agents/**/*.md` с относительными путями и значения `MANAGED_KEYS` (включая факт их отсутствия).
2. Записывается и факт существования папки `agents` и файла `settings.json`: restore должен вернуть и их отсутствие.
3. Initial нельзя редактировать и удалять; переснять можно только явно: `squad initial --recapture`, с подтверждением и бэкапом старого.

**Первый запуск: обнаружение существующего конфига**

Если у пользователя уже есть свои агенты или настройки, тула находит их до любого другого действия, сохраняет как Initial и явно показывает, что именно захвачено. Пользователь должен уйти с первого экрана с мыслью «моё не потеряется».

```text
╭─ Welcome to squadup ─────────────────────────────────────────────────────╮
│ We found your existing Claude Code setup                                │
│ and saved it as  ◇ Initial  — your safe point.                         │
│                                                                         │
│   Agents       3   reviewer · tester · docs-writer                       │
│   Model        opus                                                     │
│   Effort       high                                                     │
│   Permissions  acceptEdits · 4 allow · 1 ask · 2 deny                   │
│   Main agent   —                                                        │
│                                                                         │
│ Nothing was changed. Run  squad restore  any time to get this back.     │
╰─────────────────────────────────────────────────────────────────────────╯
  [Create first preset]   [Start from current setup]   [Browse templates]
```

- Два сценария текста: «We found your existing Claude Code setup» (есть хотя бы один агент или управляемый ключ) и «Clean Claude Code setup — Initial is the defaults» (ничего нет).
- Кнопка «Start from current setup» создаёт редактируемую копию Initial: сам Initial неизменен.
- В списке пресетов Initial всегда подписан «your original setup · captured <дата>»; пока ни один пресет не применён, он помечен как активный.
- Без TTY или при запуске команды (`squad list` и т.п.) та же сводка печатается один раз текстом, перед выводом команды.
- Агенты с ошибками (битый YAML, недопустимое имя) тоже копируются байт в байт и показываются с пометкой «1 agent has issues — run squad lint initial»: Initial хранит конфиг как есть, не чинит его.
- Переустановка тулы не трогает уже существующий `~/.squadup`. Если `~/.squadup` удалён, тула снимает Initial заново и пишет об этом: исходное состояние до прошлой установки в таком случае уже недоступно.

**`squad use <preset>`**

1. Взять lock; валидировать пресет и доступность его моделей.
2. Проверить дрейф: сравнить sha256 файлов из `state.json` с диском. Если агента правили руками — предложить: сохранить правки в текущий пресет / отбросить / отменить.
3. Построить план изменений и показать diff: агенты +/−/\~, ключи settings. `--dry-run` останавливается здесь; `--yes` пропускает подтверждение.
4. Бэкап: `~/.claude/agents` и `settings.json` → `backups/<timestamp>/`.
5. Удалить файлы, записанные предыдущим пресетом, строго по манифесту.
6. `mode: replace` — убрать агентов Initial (только файлы, идентичные снимку; копии лежат в `presets/initial`). `mode: extend` — оставить их.
7. Чужие файлы (не из Initial и не из манифеста) не трогать никогда; при совпадении имени агента — остановиться и спросить.
8. Записать агентов пресета атомарно (tmp-файл + rename), имя файла `<name>.md`.
9. Слить settings.json: каждый `MANAGED_KEY` = значение пресета, иначе значение Initial (или удаление ключа, если в Initial его не было). Остальные ключи и их порядок не меняются; отступ 2 пробела; атомарная запись.
10. Обновить `state.json`: `active`, `appliedAt`, манифест `{path: sha256}`, id бэкапа.
11. Сообщить итог. Если папки `agents` раньше не было — «restart claude»; если менялись `model` или `agent` — «applies to new sessions».

**Остальные операции**

- `squad restore` = `use initial`: удалить записанное по манифесту, вернуть файлы и ключи Initial.
- `squad undo`: вернуть состояние из последнего бэкапа и предыдущий `state.json`.
- `squad run <preset> [-- аргументы claude]`: собрать JSON для `--agents` (`prompt` = тело, без `color`), временный settings-файл в `os.TempDir()` для `--settings`, `--agent <main>` если задан; запустить `claude` с `stdin/stdout/stderr терминала`, пробросить код выхода и сигналы, удалить tmp. В `~/.claude` ничего не пишется.
- Важная оговорка про `run`: `--agents` добавляет агентов поверх `~/.claude/agents`, а не заменяет их. Это честно пишется в UI («+ your global agents»). Проверить, есть ли способ изолировать (например `--setting-sources`), и если есть — дать флаг `--isolated`.

## Доступные модели

Команды, которая выдала бы список доступных аккаунту моделей, в Claude Code нет: они видны только в пикере `/model`. Поэтому список собирается в три слоя, и каждая модель в UI помечена статусом: verified, allowed by policy или unknown.

1. **Каталог в коде.** Для агентов: `inherit`, `fable`, `opus`, `sonnet`, `haiku` + полный ID ручным вводом. Для `settings.model` дополнительно `default`, `best`, `opus[1m]`, `sonnet[1m]`, `opusplan`. Effort: `low`, `medium`, `high`, `xhigh`, `max` (на старых моделях Claude Code сам понижает уровень).
2. **Политика.** Прочитать `availableModels` (и `enforceAvailableModels`, `deniedModels`) из user- и managed-settings и применить семантику сопоставления: семейство (`sonnet`), префикс версии (`claude-sonnet-4-5`), полный ID; конкретная версия в списке отключает «всех» данного семейства. Пути managed-settings по ОС сверить по документации.
3. **Проба.** Для `fable`, `opus`, `sonnet`, `haiku` запустить `claude -p "Reply with OK" --model <alias> --output-format json` (минимальный запрос, таймаут 30 с, параллельно). Успех = модель доступна; из JSON взять фактический ID, в который резолвится алиас (нужное поле найти на реальном выводе). Проба тратит немного токенов, поэтому запускается только с согласия (первый запуск или `squad models --probe`).
4. **Кэш.** `cache/models.json` на 24 часа с ключом `claude -v` + `authMethod` из `claude auth status`; смена версии или аккаунта сбрасывает кэш.
5. **Фолбэк.** `claude` не найден или не залогинен (`auth status` → код 1) — каталог со статусом unknown и подсказка, как починить.

В UI недоступная модель серая с причиной. При `use` и `import` недоступная модель агента даёт предупреждение и предложение заменить на `inherit`. Если в окружении задан `ANTHROPIC_MODEL` или не-Anthropic провайдер (Bedrock, Vertex, Foundry) — показать это в `status`: алиасы там резолвятся в другие версии.

## Команды CLI

Исполняемый файл — `squad` (алиас `squadup`). Без аргументов открывает TUI; каждое действие TUI есть и как команда.

| Команда | Что делает |
| --- | --- |
| `squad` | Открыть TUI (главный экран) |
| `squad list` (`ls`) | Список пресетов, активный отмечен |
| `squad status` | Активный пресет, дрейф, версия `claude`, провайдер, модели |
| `squad use <preset>` | Применить в `~/.claude` (`--dry-run`, `--yes`) |
| `squad run [preset] [-- args]` | Запустить `claude` с пресетом без записи; без имени — пресет из `.squadup` в текущей папке (этап 6) |
| `squad restore` | Вернуть Initial |
| `squad undo` | Откатить последнее применение из бэкапа |
| `squad new [name]` | Создать пресет: \`--from \<preset |
| `squad capture <name>` | Сохранить текущее состояние `~/.claude` как новый пресет |
| `squad edit <preset> [agent]` | Редактор в TUI; `--raw` открывает файл в `$EDITOR` |
| `squad clone <preset> <new>` | Копия пресета |
| `squad rename <preset> <new>` | Переименование |
| `squad rm <preset>` | Удалить; `initial` нельзя, активный — только с `--force` |
| `squad export <preset> [-o file]` | Один файл `<name>.squad.json`; `-` выводит в stdout |
| \`squad import \<file | url |
| `squad models [--probe]` | Доступные модели и их статус |
| `squad lint [preset]` | Проверка агентов и настроек |
| `squad doctor` | `claude` найден, версия, логин, права на запись, целостность пресетов |
| `squad initial [--recapture]` | Показать или переснять Initial |

Дополнительные команды (подробности — в «Дополнительных фичах»):

| Команда | Что делает |
| --- | --- |
| `squad log` | История операций; `squad undo --to <id>` откатывает к точке |
| `squad card <agent>` | Карточка агента для скриншота |
| `squad pin <preset>` | Закрепить пресет за текущим проектом (файл `.squadup`) |
| `squad statusline enable`, `disable` | Показывать активный пресет в статус-строке Claude Code |
| `squad update` | Проверить новую версию и подсказать команду обновления |
| `squad completion <shell>` | Автодополнение для zsh, bash, fish (из cobra), включая имена пресетов |

Общие флаги: `--json` (машинный вывод для `list`, `status`, `models`, `lint`), `--yes`, `--no-color` (и уважение `NO_COLOR`), `--claude-dir <path>`, `--home <path>`. Коды выхода: 0 успех, 1 ошибка, 2 провал валидации или линтера, 3 отменено пользователем. Без TTY команды, требующие подтверждения, падают с подсказкой про `--yes`, а не зависают.

## Интерфейс

Интерфейс построен на игровой метафоре снаряжения, но термины остаются понятными: Presets, Agents, Initial. Игровые только глаголы и мелкие детали: **Equip** (= use), **Try** (= run), агенты как слоты. Все строки — на английском.

Главный экран (ориентир для вёрстки, 80×24):

```text
╭─ squadup ─────────────────────────────────────────────── claude 2.1.x · sonnet ─╮
│ PRESETS              │ Solo Indie                          ● EQUIPPED │
│                      │ Planner, reviewer and tester for one-person apps   │
│ ● solo-indie         │                                                   │
│ ○ bug-hunt          │  SLOT 1  ■ planner    opus    high   plan        │
│ ○ refactor-day      │  SLOT 2  ■ reviewer   sonnet  high   default     │
│ ○ ship-it           │  SLOT 3  ■ tester     haiku   medium acceptEdits │
│ ──────────────────── │  SLOT 4  + add agent                              │
│ ◇ initial           │                                                   │
│                      │  Default model  sonnet    Permissions  acceptEdits │
│                      │  Main agent     —         Lint         ✓ clean     │
╰──────────────────────┴───────────────────────────────────────────────────╯
 ↑↓ select  enter open  E equip  T try  N new  X export  I import  ? help  q quit
```

**Экраны**

1. **First run.** Логотип с градиентом и итог снимка: «We found your existing Claude Code setup (макет и правила — в разделе «Initial и переключение»)». Далее выбор: создать пресет с нуля, из шаблона или из текущего конфига; потом предложение проверить модели.
2. **Home.** Слева список пресетов (Initial всегда внизу, отделён чертой), справа карточка выбранного. При дрейфе вверху жёлтая плашка «Manual edits detected».
3. **Preset.** Слоты агентов (добавить, удалить, поменять порядок), параметры пресета: default model, effort, permission profile, main agent, mode replace/extend.
4. **Agent editor.** Форма: Name, Description (многострочно), Model (только доступные), Effort, Permission mode, Tools (чекбоксы встроенных инструментов + свои), Disallowed tools, Max turns, Color (8 цветов Claude Code), Prompt (клавиша открывает `$EDITOR`). Рядом живое превью итогового `.md` и подсказки линтера.
5. **Permission profiles.** Три готовых профиля вместо сырого JSON: Careful (`default` + deny на чтение `.env`), Balanced (`acceptEdits` + ask на `git push`), Autopilot (`auto`). Пункт Custom открывает списки allow/ask/deny. `bypassPermissions` доступен только в Custom, красным и с подтверждением.
6. **Equip confirm.** Diff (+ зелёный, − красный, \~ жёлтый), предупреждения, кнопки \[Equip\] \[Cancel\]. После — короткий «equipped»-флэш и подсказка `squad undo`.
7. **Import preview.** Состав, модели, права, предупреждения безопасности, конфликт имён.

**Правила дизайна**

- Цвета агентов = поле `color` (red, blue, green, yellow, purple, orange, pink, cyan): тот же цвет, что и внутри Claude Code. Один акцентный цвет тулы (янтарный), остальное — оттенки серого.
- Рамки со скруглёнными углами, моноширинные колонки в списках, подсказки клавиш всегда внизу.
- Минимум 80×24; шире — таблица растягивается, уже — одна колонка вместо двух.
- Клавиши: стрелки и `j`/`k`, `enter`, `esc` назад, `/` фильтр, `?` справка; буквы-действия подсвечены в футере.
- Анимации короткие (до 300 мс), спиннер только для пробы моделей и сетевого импорта.
- `NO_COLOR`, не-TTY и CI → текстовый вывод без TUI и без ANSI.

## Экспорт и импорт

Пресет уезжает одним JSON-файлом: его можно прочитать без тулы, положить в gist или репозиторий и валидировать той же схемой. Импорт никогда не применяет пресет сам.

```json
{
  "format": "squadup-preset",
  "schema": 1,
  "generator": "squadup@0.1.0",
  "exportedAt": "2026-10-23T22:10:05Z",
  "preset": { "name": "solo-indie", "title": "Solo Indie", "mode": "replace", "settings": { "model": "sonnet" } },
  "agents": [
    { "file": "reviewer.md", "content": "---\nname: reviewer\n...\n---\nYou are a strict code reviewer..." }
  ]
}
```

**Источники импорта:** локальный файл, `https://` URL (raw GitHub, raw gist; не http, лимит 1 МБ), stdin (`-`). `squad export --all` собирает массив пресетов в один файл для переезда на другую машину.

**Проверки при импорте**

| Проверка | Действие |
| --- | --- |
| Схема, `format`, `schema` новее поддерживаемой | Отказ с точной ошибкой |
| Имена файлов агентов (`../`, абсолютные пути, не `.md`) | Отказ: защита от записи вне папки пресета |
| Имя пресета уже есть | Предложить новое имя или замену (`--force`) |
| `permissionMode` или `defaultMode` = `bypassPermissions`, `dontAsk`, `auto` | Красное предупреждение, нужно явное согласие |
| Широкие allow-правила (`Bash`, `Bash(*)`, `WebFetch`) | Жёлтое предупреждение |
| Поля `hooks` и `mcpServers` в агентах (запускают команды) | Вырезаются; оставить можно только флагом `--allow-hooks` с показом команд |
| Модели, недоступные аккаунту | Предложить замену на `inherit` |
| Неизвестные поля frontmatter | Сохранить, показать линтером |

Экспорт не включает ничего из `env` и не читает секреты; перед записью показывает, что именно уйдёт в файл, если в промптах найдены строки, похожие на ключи (`sk-`, `ghp_`, `AKIA`).

## Репозиторий, установка и релизы

Репозиторий создаётся локально в `~/dev/squadup` (`git init`, ветка `main`, модуль `github.com/<owner>/squadup`) и публикуется на GitHub только после подтверждения автора. В репозитории лежит исходный код; пользователь получает один собранный файл `squad` из GitHub Releases.

```text
squadup/
  go.mod  go.sum
  cmd/squad/main.go        # точка входа: cobra-команды; без аргументов — TUI
  internal/
    cli/                   # по файлу на команду: use.go, run.go, restore.go, export.go ...
    paths/                 # CLAUDE_CONFIG_DIR, SQUADUP_HOME, --claude-dir
    claude/                # поиск бинаря, -v, auth status, запуск
    preset/                # структуры Preset/Agent, Validate(), хранилище
    agentfile/             # parse/serialize md + frontmatter, байт-точный round-trip
    settings/              # чтение и точечная правка MANAGED_KEYS (gjson/sjson)
    apply/                 # use / restore / undo, манифест, бэкапы, дрейф, lock, журнал
    models/                # каталог, политика, проба, кэш
    lint/                  # линтер агентов и делегирования, оценка токенов
    transfer/              # export / import / gist
    fsx/                   # атомарная запись, sha256, блокировка
    tui/                   # bubbletea-модели экранов, theme.go, copy.go
  templates/               # стартовые пресеты, вшиваются через embed
  plugin/                  # плагин Claude Code с командой /squad (этап 7)
  testdata/script/*.txtar  # e2e-сценарии testscript
  docs/demo.tape           # сценарий vhs для GIF
  scripts/install.sh       # установщик для curl
  .goreleaser.yaml  .golangci.yml  Makefile
  .github/workflows/ci.yml  release.yml
  README.md  LICENSE  CHANGELOG.md  CONTRIBUTING.md
```

**Установка для пользователей**

| Способ | Команда |
| --- | --- |
| Homebrew (macOS, Linux) | `brew install <owner>/tap/squadup` |
| Скрипт (macOS, Linux) | `curl -fsSL https://raw.githubusercontent.com/<owner>/squadup/main/scripts/install.sh \| sh` |
| Для Go-разработчиков | `go install github.com/<owner>/squadup/cmd/squad@latest` |
| Вручную (включая Windows) | Скачать архив из GitHub Releases, положить `squad` в PATH |
| Из исходников | `git clone … && make install` |

Короткая форма `brew install squadup` требует формулы в `homebrew/core`. Условия: стабильный тег, сборка из исходников, репозиторий старше 30 дней и заметность: минимум 75 звёзд (или 30 форков / 30 watchers), а если формулу предлагает сам автор — 225 звёзд (или 90 / 90) ([Homebrew](https://docs.brew.sh/Package-Acceptance-Policy)). До этого — свой tap; после первой команды `brew tap <owner>/tap` дальше работает и короткое `brew install squadup`. Когда порог пройден, формулу переносим в core и сначала проверяем, что имя там свободно.

`install.sh` определяет ОС и архитектуру, скачивает нужный архив из последнего релиза, проверяет чексумму и кладёт бинарь в `~/.local/bin` (или `/usr/local/bin` с вопросом), подсказывает, если папки нет в PATH. `squad update` проверяет новую версию и подсказывает команду обновления для способа, которым ставили.

README начинается с GIF и трёх строк: install, `squad`, `squad use <preset>`. Далее — что тула трогает в `~/.claude` и чего не трогает никогда, как откатиться, формат пресета.

**CI и релизы**

- `ci.yml`: `golangci-lint`, `go vet`, `go test -race` на macOS, Ubuntu и Windows.
- `release.yml`: по тегу `v*` GoReleaser собирает darwin/linux/windows × amd64/arm64, выкладывает GitHub Release с чексуммами и чейнджлогом, обновляет формулу в репозитории `<owner>/homebrew-tap`.
- Версия вшивается через `-ldflags`; `squad --version` показывает версию, коммит и дату сборки.
- Версии по semver; `schema` пресета версионируется отдельно, с миграциями вперёд.

## Тесты и безопасность

Тула пишет в живой конфиг разработчика, поэтому главный тест — «golden round-trip»: после любой цепочки `use` и `restore` дерево файлов совпадает с исходным байт в байт.

**Тесты**

- Изоляция: каждый тест работает в `t.TempDir()` с заданными `SQUADUP_HOME` и `CLAUDE_CONFIG_DIR`. Предохранитель в `internal/paths`: в тестовой сборке запись в настоящий `~/.claude` вызывает panic.
- Заглушка `claude`: небольшая Go-программа в `testdata`, подкладывается в `PATH`; отвечает на `-v`, `auth status`, `-p --model` и пишет полученные аргументы в файл для проверки `run`.
- Unit (табличные тесты): round-trip агента (parse → serialize = те же байты), правка settings не меняет чужие ключи, их порядок и отступы, сопоставление `availableModels`, линтер, оценка токенов, валидация импорта (включая path traversal).
- E2E на testscript (`testdata/script/*.txtar`): первый запуск с чужим конфигом → Initial и сводка; первый запуск на пустом `~/.claude`; `use A` → `use B` → `restore` = исходное дерево; ручная правка → дрейф; чужой файл агента переживает любые операции; `undo`; импорт опасного пресета; `run` передаёт правильные флаги.
- TUI: `teatest` — золотые снэпшоты ключевых экранов на фиксированном размере 80×24 и сценарий нажатий для Equip.
- Ручная проверка перед релизом: пресет применён при открытой сессии Claude Code, новый агент виден без рестарта (на тестовом `CLAUDE_CONFIG_DIR`); установка через brew и `install.sh` на чистой машине.

**Безопасность**

- Любая запись в `~/.claude`: lock → бэкап → атомарная запись → обновление манифеста. Падение на середине откатывается из бэкапа.
- Удаляются только файлы из манифеста или байт-идентичные снимку Initial.
- `env`, `hooks`, `enabledPlugins`, `~/.claude.json` — не читаются в логи и не пишутся; `statusLine` — только по явному `squad statusline enable`.
- Код из импортированных пресетов никогда не выполняется. Сеть: только `https`-импорт, gist через `gh` и проверка обновлений (отключаемая). Телеметрии нет.

## Этапы

Восемь этапов (0–7) до v0.1.0, все дополнительные фичи входят в них. После каждого этапа — коммит, зелёный CI и короткий отчёт автору.

| Этап | Объём | Готово, когда |
| --- | --- | --- |
| 0. Скелет | `go mod init`, cobra, Makefile, golangci-lint, CI, GoReleaser (локальная snapshot-сборка), `squad --version`, `squad doctor` | `make install` → `squad doctor` находит `claude`, версию и статус логина |
| 1. Ядро | `paths`, `preset`, `agentfile`, обнаружение конфига и снимок Initial со сводкой, `list`, `status`, `capture`, `initial` | Первый запуск на чужом конфиге печатает правильную сводку; round-trip агентов байт в байт |
| 2. Применение | `use`, `restore`, `undo`, манифест, бэкапы, lock, дрейф, правка settings, `--dry-run`, журнал `squad log` | Golden-тест `use A → use B → restore` зелёный на трёх ОС |
| 3. Run, модели, линтер | `run`, `models` (каталог + политика + проба + кэш), `lint` с линтером делегирования, оценка стоимости пресета | `squad run` запускает настоящий `claude` с агентами пресета; линтер ловит все правила из раздела ниже |
| 4. TUI | First run, Home, Preset, Agent editor, Permission profiles, Equip confirm, `new`, `edit`, `clone`, `rename`, `rm`; стоимость и линт на карточке пресета | Весь сценарий без единого флага; работает в 80×24 и с `NO_COLOR` |
| 5. Обмен и шаблоны | `export`, `import`, gist, Import preview, проверки безопасности, 5 стартовых шаблонов, `squad card` | Экспорт → импорт в чистый `SQUADUP_HOME` даёт идентичный пресет |
| 6. Интеграции | Файл `.squadup` в проекте, `squad statusline`, плагин Claude Code с `/squad` | `/squad` в сессии Claude Code показывает и переключает пресет; статус-строка включается и отключается без потерь |
| 7. Релиз v0.1.0 | README с GIF (vhs), LICENSE, CHANGELOG, `install.sh`, Homebrew tap, локальная сборка релиза (goreleaser --snapshot) без публикации | `brew install` и `install.sh` на чистой машине проходят путь от установки до Equip и Restore |

**Все этапы 0–7 выполняются только локально.** Критерий этапа 7 проверяется на локальных архивах из `dist/`: `install.sh` умеет ставить из локального файла (`SQUADUP_INSTALL_FROM=./dist/...`). Публикация — отдельный этап 8 после того, как автор посмотрит код и потестирует: создать GitHub-репозитории `squadup` и `homebrew-tap`, добавить секрет с токеном, запушить, поставить тег `v0.1.0`. Этап 8 начинается только по явной команде автора.

После v0.1: проектный scope (`.claude/agents` в репозитории), скиллы в пресетах, Scoop/WinGet для Windows.

## Дополнительные фичи

Все одобрены автором и разложены по этапам выше.

| Фича | Как работает | Этап |
| --- | --- | --- |
| Линтер делегирования | Правила: неизвестное поле и похожее известное (`disallowed_tools` → `disallowedTools`); недопустимое имя и дубли; `description` короче 40 символов или без «когда вызывать» (нет слов use / when / after / before); два агента с почти одинаковыми описаниями; пустой промпт; `main`, которого нет среди агентов. Уровни error / warn / info | 3 |
| Стоимость пресета | Приблизительная оценка токенов, которые имена и описания агентов занимают в каждой сессии (символы ÷ 4), с пометкой «\~»; для агента отдельно — размер промпта, который грузится при вызове | 3 |
| Журнал | `~/.squadup/log.jsonl`: каждая операция (время, команда, пресет, id бэкапа). `squad log` показывает историю, `squad undo --to <id>` откатывает к любой точке | 2 |
| Карточка агента | `squad card <agent> [--preset p]`: рамка lipgloss в цвете агента — имя, роль (первая фраза description), модель, effort, инструменты, «сила прав» шкалой. Для скриншотов и соцсетей | 5 |
| Gist | `squad export <p> --gist` через установленный `gh` (если нет — понятная ошибка); `squad import gist:<id>` | 5 |
| Стартовые шаблоны | Solo Indie (planner, reviewer, tester), Bug Hunt (debugger, tester), Refactor Day (architect, refactorer, reviewer), Ship It (release-checker, changelog-writer), Read-only Audit (security-auditor, reviewer; только Read/Grep/Glob). Промпты — конкретные, с форматом ответа; каждый шаблон проходит `squad lint` без предупреждений | 5 |
| Пресет на проект | `squad pin <preset>` пишет файл `.squadup` (имя пресета) в текущую папку; `squad run` без аргумента ищет ближайший `.squadup` вверх по дереву; `squad status` пишет, если активный пресет не совпадает с закреплённым | 6 |
| Статус-строка | `squad statusline enable` сохраняет текущий `statusLine` пользователя и ставит команду `squad statusline`; та запускает прежнюю команду с тем же stdin и дописывает «◆ solo-indie». `disable` возвращает как было. Формат `statusLine` и входного JSON сверить по документации | 6 |
| Плагин Claude Code | Папка `plugin/` (`.claude-plugin/plugin.json` + команда или скилл `/squad`) и `marketplace.json` в корне репозитория. `/squad` вызывает `squad status --json` / `squad list --json` и по просьбе `squad use <p> --yes`; если бинаря нет — показывает команду установки. Сам плагин не содержит агентов, чтобы не тратить контекст. Формат плагина сверить по документации | 6 |

## Решения автора

Все вопросы закрыты 23.10.2026; открытых решений нет.

| Вопрос | Решение |
| --- | --- |
| Имя | `squadup`, команда `squad` |
| Стек | Go + Charm, один бинарь, установка через Homebrew и `install.sh` |
| Режим пресета | replace: пресет заменяет агентов Initial |
| Windows | «Должно работать»: бинарь собирается и тестируется в CI, но официальной поддержки нет |
| Обнаружение существующего конфига | Обязательно при первом запуске, со сводкой (см. «Initial и переключение») |
| Дополнительные фичи | Реализуем все, по этапам 2–6 |
| Проектный scope (`.claude/agents`) | После v0.1 |
| Проба моделей | С согласия пользователя |
| Термины в UI | Equip / Try на кнопках, `use` / `run` в командах |

## Локальная проверка автором

Автор смотрит код и тестирует до любой публикации, поэтому в репозитории с этапа 0 есть безопасная песочница, а в README — раздел «Local development».

| Команда | Что делает |
| --- | --- |
| `make build` | Собрать `./bin/squad` |
| `make test` | Все тесты + линтер |
| `make sandbox` | Создать `./.sandbox/claude` с примером чужого конфига (3 агента, settings.json с правами и чужими ключами) и пустой `./.sandbox/squadup` |
| `make try` | Запустить `squad` с `CLAUDE_CONFIG_DIR` и `SQUADUP_HOME`, указывающими в песочницу; аргументы передаются: `make try ARGS="use solo-indie"` |
| `make try-claude` | Открыть настоящий `claude` на песочнице, чтобы увидеть агентов пресета внутри Claude Code (нужен логин; сверить, как логин ведёт себя при другом `CLAUDE_CONFIG_DIR`) |
| `make sandbox-reset` | Сбросить песочницу |
| `make snapshot` | `goreleaser release --snapshot --clean`: архивы для всех ОС в `dist/` без публикации |
| `make install` | Скопировать `squad` в `~/.local/bin` для проверки на настоящем конфиге (первый запуск снимет Initial, `squad restore` вернёт всё как было) |

`.sandbox/`, `bin/`, `dist/` — в `.gitignore`. После каждого этапа исполнитель даёт автору 3–5 команд для ручной проверки сделанного.

## Правила для агента-исполнителя

Этот раздел — контракт. Если правило конфликтует с удобством, побеждает правило.

1. Прочитать план целиком до первой строки кода. Работать строго по этапам; после каждого — коммит и отчёт: что сделано, что проверено, что отложено.
2. Никогда не писать в настоящий `~/.claude` автора во время разработки. Ручные проверки — только с `CLAUDE_CONFIG_DIR` и `SQUADUP_HOME`, указывающими на временные папки.
3. Перед использованием флага или поля Claude Code свериться с `claude --help` на машине и с документацией на code.claude.com. Особенно: поле JSON с фактической моделью в `-p --output-format json`, пути managed-settings, формат `statusLine`, способ изолировать `--agents` от глобальных агентов. Расхождение с планом — записать в `docs/DECISIONS.md` и сообщить.
4. Все строки интерфейса, сообщения об ошибках, README и комментарии в коде — на английском. Тексты UI вынести в `internal/tui/copy.go`.
5. Ошибка для пользователя = что случилось + как починить, без стектрейса (стектрейс — под `DEBUG=squadup`).
6. Зависимости — только из раздела «Ключевые решения», актуальные стабильные версии, lock-файл в репозитории. Новая зависимость — с обоснованием в отчёте.
7. Работа только локальная: только локальные git-коммиты. Запрещено до явной команды автора: \`git remote add\`, \`git push\`, \`gh repo create\`, любые вызовы \`gh\`, создание тегов релиза, \`goreleaser release\` без \`--snapshot\`, публикация в любой реестр. Путь модуля Go — \`github.com/\<owner>/squadup\`, где \`\<owner>\` — GitHub-логин автора (спросить на этапе 0, если не указан).
8. Этап сдан, когда выполнен критерий из таблицы «Этапы», проходят lint, typecheck и тесты, и нет незакрытых TODO без issue.

## Источники

- [Claude Code: subagents](https://code.claude.com/docs/en/subagents) — поля frontmatter, `--agents`, приоритеты, горячая перезагрузка
- [Claude Code: CLI reference](https://code.claude.com/docs/en/cli-reference) — флаги `--agent`, `--agents`, `--settings`, `--permission-mode`, `auth status`
- [Claude Code: model configuration](https://code.claude.com/docs/en/model-config.md) — алиасы, `availableModels`, effort
- [Claude Code: settings reference](https://code.claude.com/docs/en/settings-reference) — ключи settings.json, синтаксис прав
- [Claude Code: .claude directory](https://code.claude.com/docs/en/claude-directory.md) — `CLAUDE_CONFIG_DIR`, содержимое `~/.claude`
- npm registry, проверка имён 23.10.2026: `squadup` свободно; `loadout`, `claude-loadout`, `agent-loadout`, `claude-squad`, `claude-presets` заняты
