# 🧠 Архитектура памяти проекта "Мира"

> **Примечание:** Этот документ описывает систему хранения данных для восходящего ИИ "Мира". Архитектура рассчитана на биологический масштаб (86 миллиардов нейронов, 130 триллионов синапсов) при жёстких ограничениях по памяти (512 ГБ SSD, 64 ГБ RAM). Используется гибридный подход: процедурная связность + sparse delta-лог с LZ4-сжатием.

---

## 📑 Содержание

- [Общая концепция](#-общая-концепция)
- [Биологическая модель нейрона](#-биологическая-модель-нейрона-adex)
- [Квантование параметров](#-квантование-параметров)
- [Структура хранения](#-структура-хранения)
- [Файл base.mcog](#-файл-basemcog-процедурный-слой)
- [Файл delta.wal](#-файл-deltawal-слой-модификаций)
- [Математика объёмов](#-математика-объёмов)
- [Алгоритмы работы](#️-алгоритмы-работы)
- [Реализация на Go](#-реализация-на-go)
- [Примеры кода](#-примеры-кода)
- [Оптимизации](#-оптимизации)
- [Компромиссы](#️-компромиссы)
- [RAM-стратегия](#-ram-стратегия)

---

## 🎯 Общая концепция

### Проблема масштабирования

При биологическом масштабе (86 млрд нейронов, 130 трлн синапсов) явное хранение всех синапсов в формате CSR требует **617 ТБ** дискового пространства. Это физически невозможно на одном SSD.

### Решение: Парадигма Nature vs Nurture

В биологии **>95% синапсов формируются по генетическим правилам** (nature), и лишь малая часть модифицируется опытом (nurture). Мы используем это разделение:

1. **Процедурный слой (Nature):** Храним не сами синапсы, а правила их генерации. Когда нужны синапсы нейрона, они генерируются на лету через детерминированный PRNG.

2. **Delta-слой (Nurture):** Храним только отклонения от процедурного плана — синапсы, созданные обучением, удалённые синапсы, модифицированные веса.

### Архитектура v3 (Human-Scale)

```plaintext
┌─────────────────────────────────────────────────────────┐
│                    Mira Memory System                   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────┐              ┌──────────────┐         │
│  │  base.mcog   │              │  delta.wal   │         │
│  │  (Read-Only) │              │ (Append-Only)│         │
│  │              │              │              │         │
│  │ • Нейроны    │              │ • Нейрогенез │         │
│  │ • Seeds      │              │ • Синаптогенез│        │
│  │ • Clusters   │              │ • STDP       │         │
│  │ • Codebook   │              │ • Удаления   │         │
│  └──────┬───────┘              └──────┬───────┘         │
│         │                             │                  │
│         │   При чтении синапсов       │                  │
│         │   происходит слияние        │                  │
│         └─────────────┬───────────────┘                  │
│                       │                                  │
│                       ▼                                  │
│         ┌──────────────────────────┐                    │
│         │ Virtual Synapse Iterator │                    │
│         │ (объединяет Base+Delta)  │                    │
│         └──────────────────────────┘                    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Ключевые преимущества:**

- ✅ Человеческий масштаб (86 млрд нейронов) на одном SSD (512 ГБ)
- ✅ Быстрый старт (загрузка через mmap за ~1 секунду)
- ✅ Эффективное хранение (тратим память только на обученное)
- ✅ Динамический нейрогенез без пересборки файлов

---

## 🔬 Биологическая модель нейрона (AdEx)

### Почему не Hodgkin-Huxley?

Модель Ходжкина-Хаксли требует ~50 переменных на компартмент и 1000+ компартментов на нейрон. Это **мегабайты на нейрон** — нереально для миллиардов нейронов.

### Adaptive Exponential Integrate-and-Fire (AdEx)

Биологически правдоподобная модель с минимальным набором переменных:

```math
C \frac{dV}{dt} = -g_L(V - E_L) + g_L \Delta_T \exp\left(\frac{V - V_T}{\Delta_T}\right) - w + I
```

```math
\tau_w \frac{dw}{dt} = a(V - E_L) - w
```

**Переменные состояния:**

- `V` — мембранный потенциал (мВ)
- `w` — адаптационный ток (нА)

**Статические параметры:**

- `threshold` — порог спайка (мВ)
- `resting_potential` — потенциал покоя (мВ)
- `neuron_type` — тип нейрона (пирамидальный, интернейрон и т.д.)

**Событие спайка:**

- Когда `V > threshold`, происходит спайк
- `V` сбрасывается к `resting_potential`
- `w` увеличивается на `b` (spike-triggered adaptation)

### Преимущества AdEx

- ✅ Биологическая правдоподобность (воспроизводит 12 из 15 типов firing patterns)
- ✅ Минимум переменных (2 динамические + 5 статических)
- ✅ Быстрое вычисление (ODE решается за ~10 тактов CPU)
- ✅ Поддержка STDP (пластичности)

---

## 📐 Квантование параметров

### Мембранный потенциал (V)

**Диапазон:** -90 мВ до +40 мВ = 130 мВ
**Биологическая точность:** порог спайка меняется на ~10 мВ, шум ~0.1 мВ
**Квантование:** 12 бит → 4096 уровней
**Точность:** 130 / 4096 ≈ **0.03 мВ** (меньше теплового шума)
**Экономия:** 32 бита (float32) → 12 бит = **сжатие 2.67×**

### Адаптационный ток (w)

**Диапазон:** 0 до 10 нА
**Квантование:** 8 бит → 256 уровней
**Точность:** 10 / 256 ≈ **0.04 нА**
**Экономия:** 32 бита → 8 бит = **сжатие 4×**

### Порог спайка (threshold)

**Диапазон:** -60 до -40 мВ (медленно меняется, гомеостаз)
**Квантование:** 8 бит → 256 уровней
**Точность:** 20 / 256 ≈ **0.08 мВ**
**Экономия:** 32 бита → 8 бит = **сжатие 4×**

### Потенциал покоя (resting_potential)

**Диапазон:** -75 до -55 мВ
**Квантование:** 8 бит → 256 уровней
**Точность:** 20 / 256 ≈ **0.08 мВ**
**Экономия:** 32 бита → 8 бит = **сжатие 4×**

### Тип нейрона (neuron_type)

**Варианты:** пирамидальный, интернейрон, клетка Пуркинье, зернистая клетка и т.д.
**Квантование:** 4 бита → 16 типов
**Экономия:** 32 бита → 4 бита = **сжатие 8×**

### Seed для PRNG

**Назначение:** детерминированная генерация синапсов
**Диапазон:** 0 до 65535 (достаточно для уникальности)
**Размер:** 16 бит
**Экономия:** vs хранение всех синапсов явно = **бесконечная**

### Cluster ID

**Назначение:** идентификация кластера (микроколонки коры)
**Диапазон:** 0 до 65535 (65536 кластеров)
**Размер:** 16 бит

### Итоговая упаковка нейрона

| Параметр          | Биты   | Байты   |
| ----------------- | ------ | ------- |
| seed              | 16     | 2.0     |
| cluster_id        | 16     | 2.0     |
| type              | 4      | 0.5     |
| threshold         | 8      | 1.0     |
| resting_potential | 8      | 1.0     |
| **Итого**         | **52** | **6.5** |

**С битовой упаковкой блоков:** 4.5 байта на нейрон (выравнивание до границ)

**Для 86 млрд нейронов:**

```plaintext
86 × 10⁹ × 4.5 байта = 387 ГБ
```

---

## 📦 Структура хранения

### Два слоя данных

```plaintext
data/
├── memory/
│   ├── base.mcog          # Процедурный слой (Read-Only, mmap)
│   ├── delta.wal          # Слой модификаций (Append-Only, LZ4)
│   ├── delta_new.wal      # Временный буфер во время compaction
│   ├── codebook.bin       # Эталонные веса синапсов (256 × float32)
│   └── metadata.json      # Метаданные (версии, статистика)
└── cache/
    ├── procedural_cache/  # LRU-кэш процедурно сгенерированных синапсов
    └── delta_cache/       # RAM-индекс последних записей Delta
```

### Почему два файла?

1. **base.mcog** — Read-Only, memory-mapped
   - Не меняется во время работы Миры
   - Содержит стартовую структуру мозга
   - Быстрый доступ через mmap (OS page cache)

2. **delta.wal** — Append-Only, буферизованная запись
   - Сюда пишутся все изменения (нейрогенез, STDP, синаптогенез)
   - Никогда не перезаписывается, только дописывается
   - Сжимается LZ4 для экономии места

### Compaction (фоновое слияние)

Когда Delta достигает ~50-100 ГБ или фрагментация чтения растёт:

1. Запускается фоновый поток (низкий приоритет)
2. Читает `base.mcog` + `delta.wal`
3. Применяет все модификации, удаляет stale-записи
4. Пишет `base_new.mcog` с новой структурой
5. Атомарно переименовывает `base_new.mcog` → `base.mcog`
6. Очищает `delta.wal`

**Важно:** Во время компактификации Мира продолжает работать. Новые записи идут в `delta_new.wal`, который потом применится к следующей итерации.

---

## 📄 Файл base.mcog (процедурный слой)

### Структура файла

```plaintext
[Header] (64 байта)
[Codebook] (256 × float32 = 1 КБ)
[Section Table] (64 байта × N_sections)
[Neuron Static Params] (SoA, квантованные, 4.5 байта/нейрон)
[Connectivity Rules] (правила связности между кластерами)
[Cluster Metadata] (координаты, типы, размеры кластеров)
```

### Header (64 байта)

```go
type MCogHeader struct {
    Magic           [4]byte    // "MCOG"
    Version         uint32     // Версия формата
    TotalNeurons    uint64     // Общее количество нейронов
    TotalClusters   uint32     // Количество кластеров
    SectionCount    uint32     // Количество разделов
    CodebookOffset  uint64     // Смещение до codebook
    NeuronsOffset   uint64     // Смещение до массива нейронов
    RulesOffset     uint64     // Смещение до правил связности
    ClustersOffset  uint64     // Смещение до метаданных кластеров
    Checksum        uint32     // CRC32 для проверки целостности
    Padding         [8]byte    // Выравнивание до 64 байт
}
```

### Codebook (1 КБ)

```go
type Codebook [256]float32
```

Эталонные веса синапсов. Веса в base.mcog хранятся как индексы (8 бит) в этот codebook.

**Генерация codebook:**

```go
func GenerateCodebook() [256]float32 {
    var cb [256]float32
    // Логнормальное распределение (большинство слабых, несколько сильных)
    for i := 0; i < 256; i++ {
        // μ = -1.0, σ = 0.8 (типичные значения для коры)
        cb[i] = float32(math.Exp(rand.NormFloat64()*0.8 - 1.0))
    }
    return cb
}
```

### Neuron Static Params (SoA формат)

Вместо AoS (Array of Structures) используем SoA (Structure of Arrays) для cache-friendly доступа:

```go
type NeuronParams struct {
    Seeds          []uint16  // 86 млрд × 2 байта = 172 ГБ
    ClusterIDs     []uint16  // 86 млрд × 2 байта = 172 ГБ
    Types          []uint8   // 86 млрд × 0.5 байта = 43 ГБ (упаковано)
    Thresholds     []uint8   // 86 млрд × 1 байт = 86 ГБ
    RestingPots    []uint8   // 86 млрд × 1 байт = 86 ГБ
}
```

**Упаковка полей (bit-packing):**

```go
// Types, Thresholds, RestingPots упакованы в один массив
type PackedParams struct {
    Data []uint8  // 86 млрд × 2.5 байта = 215 ГБ
}

// Извлечение полей
func (p *PackedParams) GetType(neuronID uint64) uint8 {
    byteOffset := neuronID * 5 / 2  // 2.5 байта на нейрон
    bitOffset := (neuronID % 2) * 4
    return (p.Data[byteOffset] >> bitOffset) & 0x0F
}

func (p *PackedParams) GetThreshold(neuronID uint64) uint8 {
    byteOffset := neuronID * 5 / 2
    if neuronID%2 == 0 {
        return (p.Data[byteOffset] >> 4) & 0x0F
    }
    return p.Data[byteOffset] & 0x0F
}
```

### Connectivity Rules

Правила связности между кластерами (вероятности соединения):

```go
type ConnectivityRule struct {
    SourceCluster uint16
    TargetCluster uint16
    Probability   float32  // Вероятность соединения
    DistanceFunc  uint8    // Тип дистанционной функции
}
```

**Пример правил:**

- Пирамидальные нейроны в одном кластере: P = 0.15
- Пирамидальные → интернейроны: P = 0.08
- Между далёкими кластерами: P = 0.001 (экспоненциальное затухание)

### Cluster Metadata

```go
type ClusterMeta struct {
    ID           uint16
    X, Y, Z      float32  // 3D-координаты центра
    NeuronCount  uint32   // Количество нейронов в кластере
    LayerID      uint8    // Слой коры (1-6)
    Type         uint8    // Тип кластера
}
```

---

## 📝 Файл delta.wal (слой модификаций)

### Структура записи

```plaintext
[Record Header] (8 байт)
  - type: uint8 (0=neurogenesis, 1=synaptogenesis, 2=weight_update, 3=delete)
  - timestamp: uint32 (тики с начала работы)
  - payload_len: uint24 (длина payload в байтах)
[Payload] (variable length, выровнен по 8 байт)
```

### Типы записей

#### 0: Neurogenesis (создание нового нейрона)

```go
type NeurogenesisRecord struct {
    ParentID     uint32  // ID родительского нейрона (для наследования типа)
    ClusterID    uint16  // Кластер, куда добавляется нейрон
    Seed         uint16  // Seed для PRNG
    Type         uint8   // Тип нейрона
    Threshold    uint8   // Порог спайка
    RestingPot   uint8   // Потенциал покоя
}
```

**Размер:** 10 байт + padding = 16 байт

#### 1: Synaptogenesis (создание нового синапса)

```go
type SynaptogenesisRecord struct {
    SourceID     uint32  // ID пресинаптического нейрона
    TargetID     uint32  // ID постсинаптического нейрона
    WeightIdx    uint8   // Индекс в codebook (0-255)
    Delay        uint8   // Задержка (0-15 мс)
    ReceptorType uint8   // Тип рецептора (AMPA, NMDA, GABA и т.д.)
}
```

**Размер:** 10 байт + padding = 16 байт

#### 2: Weight Update (обновление веса через STDP)

```go
type WeightUpdateRecord struct {
    SourceID     uint32
    TargetID     uint32
    OldWeightIdx uint8
    NewWeightIdx uint8
    Delta        int8    // Изменение веса (для проверки)
}
```

**Размер:** 11 байт + padding = 16 байт

#### 3: Delete (удаление синапса)

```go
type DeleteRecord struct {
    SourceID     uint32
    TargetID     uint32
    Reason       uint8   // Причина (pruning, injury и т.д.)
}
```

**Размер:** 9 байт + padding = 16 байт

### LZ4-сжатие

Delta-лог сжимается блоками по 64 КБ через LZ4:

```go
import "github.com/pierrec/lz4/v4"

func CompressDeltaBlock(data []byte) []byte {
    var buf bytes.Buffer
    writer := lz4.NewWriter(&buf)
    writer.Write(data)
    writer.Close()
    return buf.Bytes()
}
```

**Коэффициент сжатия:** 3-5× (зависит от повторяемости данных)

**Эффективный объём Delta:**

```plaintext
120 ГБ × 4 (сжатие) = 480 ГБ эффективного хранилища
480 ГБ / 16 байт = 30 млрд записей
```

Это ~350 модификаций на нейрон за всю жизнь обучения — реалистично!

### Буферизация записи

```go
type DeltaWriter struct {
    file        *os.File
    buffer      []byte
    bufferSize  int
    mu          sync.Mutex
}

func (w *DeltaWriter) Write(record []byte) error {
    w.mu.Lock()
    defer w.mu.Unlock()

    w.buffer = append(w.buffer, record...)

    // Сбрасываем буфер при достижении 64 КБ
    if len(w.buffer) >= w.bufferSize {
        return w.flush()
    }
    return nil
}

func (w *DeltaWriter) flush() error {
    compressed := CompressDeltaBlock(w.buffer)
    _, err := w.file.Write(compressed)
    w.buffer = w.buffer[:0]
    return err
}
```

---

## 📊 Математика объёмов

### Бюджет: 512 ГБ SSD

| Компонент           | Размер     | Комментарий                   |
| ------------------- | ---------- | ----------------------------- |
| base.mcog (нейроны) | 387 ГБ     | 86 млрд × 4.5 байта           |
| base.mcog (правила) | 1 ГБ       | Connectivity rules + clusters |
| Codebook            | 1 КБ       | 256 × float32                 |
| delta.wal (сжатый)  | 120 ГБ     | ~30 млрд записей              |
| Метаданные          | 1 ГБ       | Индексы, статистика           |
| Резерв              | 3 ГБ       | Для ОС и временных файлов     |
| **Итого**           | **512 ГБ** | ✅ Влезает с запасом          |

### RAM-потребление: 64 ГБ

| Компонент                | Стратегия                        | Потребление              |
| ------------------------ | -------------------------------- | ------------------------ |
| base.mcog                | mmap + OS page cache             | 10-15 ГБ (горячие зоны)  |
| delta.wal                | mmap + LZ4 декомпрессия          | 5-8 ГБ (активные записи) |
| Procedural Cache         | LRU для сгенерированных синапсов | 4-6 ГБ                   |
| Active Neuron State      | Только для 5% активных нейронов  | 3-5 ГБ                   |
| Delta Index              | RAM-индекс последних записей     | 2-4 ГБ                   |
| VirtualBox / Рендер / ОС | Резерв                           | 30-35 ГБ                 |
| **Итого**                |                                  | **~50-60 ГБ**            |

**Вывод:** Стабильно влезает в 64 ГБ RAM без swap.

---

## ⚙️ Алгоритмы работы

### Чтение синапсов нейрона

```go
func (m *MemorySystem) GetSynapses(neuronID uint32) []Synapse {
    // 1. Генерируем процедурные синапсы
    procedural := m.generateProceduralSynapses(neuronID)

    // 2. Ищем модификации в Delta
    modifications := m.findDeltaModifications(neuronID)

    // 3. Объединяем (Delta перекрывает Base)
    return m.mergeSynapses(procedural, modifications)
}
```

### Процедурная генерация синапсов

```go
func (m *MemorySystem) generateProceduralSynapses(neuronID uint32) []Synapse {
    seed := m.base.Neurons.Seeds[neuronID]
    clusterID := m.base.Neurons.ClusterIDs[neuronID]

    // Детерминированный PRNG (PCG32)
    rng := NewPCG32(seed)

    var synapses []Synapse

    // Для каждого потенциального целевого кластера
    for _, targetCluster := range m.getConnectedClusters(clusterID) {
        // Получаем вероятность соединения
        probability := m.base.Rules.GetProbability(clusterID, targetCluster)

        // Генерируем синапсы с этой вероятностью
        targetNeurons := m.base.Clusters[targetCluster].NeuronCount
        for i := uint32(0); i < targetNeurons; i++ {
            if rng.NextFloat32() < probability {
                targetID := m.base.Clusters[targetCluster].FirstNeuronID + i

                // Генерируем вес процедурно
                weightIdx := hashCombine(seed, targetID) % 256

                synapses = append(synapses, Synapse{
                    TargetID:  targetID,
                    WeightIdx: uint8(weightIdx),
                    Delay:     uint8(rng.Next() % 16),
                    Type:      uint8(rng.Next() % 4),
                })
            }
        }
    }

    return synapses
}
```

### Поиск модификаций в Delta

```go
func (m *MemorySystem) findDeltaModifications(neuronID uint32) []DeltaRecord {
    var records []DeltaRecord

    // Проверяем RAM-индекс (быстро)
    if idx, ok := m.deltaIndex[neuronID]; ok {
        records = append(records, idx.Records...)
    }

    // Сканируем Delta-файл (медленно, но полно)
    m.deltaReader.Scan(func(record DeltaRecord) bool {
        if record.SourceID == neuronID {
            records = append(records, record)
        }
        return true
    })

    return records
}
```

### Объединение Base + Delta

```go
func (m *MemorySystem) mergeSynapses(procedural []Synapse, modifications []DeltaRecord) []Synapse {
    // Создаём карту для быстрого поиска
    synapseMap := make(map[uint32]Synapse)

    // Добавляем процедурные синапсы
    for _, s := range procedural {
        synapseMap[s.TargetID] = s
    }

    // Применяем модификации (Delta перекрывает Base)
    for _, mod := range modifications {
        switch mod.Type {
        case Synaptogenesis:
            synapseMap[mod.TargetID] = Synapse{
                TargetID:  mod.TargetID,
                WeightIdx: mod.WeightIdx,
                Delay:     mod.Delay,
                Type:      mod.ReceptorType,
            }
        case WeightUpdate:
            if s, ok := synapseMap[mod.TargetID]; ok {
                s.WeightIdx = mod.NewWeightIdx
                synapseMap[mod.TargetID] = s
            }
        case Delete:
            delete(synapseMap, mod.TargetID)
        }
    }

    // Конвертируем карту обратно в срез
    result := make([]Synapse, 0, len(synapseMap))
    for _, s := range synapseMap {
        result = append(result, s)
    }

    return result
}
```

---

## 🐹 Реализация на Go

### Почему Go?

1. **Нативная компиляция** — быстрее Python, сравнимо с C++
2. **Goroutines** — отличная поддержка concurrency для параллельной работы с mmap
3. **Memory safety** — без segfaults, но с `unsafe` для низкоуровневого доступа
4. **Простой FFI** через cgo — интеграция с C-библиотеками (LZ4)
5. **Встроенные инструменты** — профилирование, бенчмарки, race detector

### Структура проекта (Go)

```plaintext
libs/
├── go/
│   ├── cmd/
│   │   └── mira-memory/
│   │       └── main.go          # Точка входа
│   ├── internal/
│   │   ├── memory/
│   │   │   ├── base.go          # Работа с base.mcog
│   │   │   ├── delta.go         # Работа с delta.wal
│   │   │   ├── procedural.go    # Процедурная генерация
│   │   │   ├── compaction.go    # Фоновое слияние
│   │   │   └── types.go         # Типы данных
│   │   ├── mmap/
│   │   │   └── mmap.go          # Обёртка над syscall.Mmap
│   │   ├── prng/
│   │   │   └── pcg32.go         # Детерминированный PRNG
│   │   └── lz4/
│   │       └── lz4.go           # Сжатие через cgo
│   ├── pkg/
│   │   └── api/
│   │       └── api.go           # Публичный API для Python
│   ├── go.mod
│   ├── go.sum
│   └── Makefile
```

### Работа с mmap

```go
package mmap

import (
    "os"
    "syscall"
)

type MMap struct {
    data []byte
    file *os.File
}

func Open(filename string, size int64) (*MMap, error) {
    file, err := os.OpenFile(filename, os.O_RDWR, 0644)
    if err != nil {
        return nil, err
    }

    // Memory mapping
    data, err := syscall.Mmap(
        int(file.Fd()),
        0,
        int(size),
        syscall.PROT_READ|syscall.PROT_WRITE,
        syscall.MAP_SHARED,
    )
    if err != nil {
        file.Close()
        return nil, err
    }

    return &MMap{data: data, file: file}, nil
}

func (m *MMap) Close() error {
    if err := syscall.Munmap(m.data); err != nil {
        return err
    }
    return m.file.Close()
}

func (m *MMap) Data() []byte {
    return m.data
}
```

### Bit-packing через unsafe

```go
package memory

import "unsafe"

type NeuronArray struct {
    data []byte
}

func NewNeuronArray(mmap *MMap, offset, size int) *NeuronArray {
    return &NeuronArray{
        data: mmap.Data()[offset : offset+size],
    }
}

// Получение seed нейрона (16 бит)
func (n *NeuronArray) GetSeed(neuronID uint64) uint16 {
    byteOffset := neuronID * 6 // 6 байт на нейрон (упаковано)
    return *(*uint16)(unsafe.Pointer(&n.data[byteOffset]))
}

// Получение cluster_id (16 бит)
func (n *NeuronArray) GetClusterID(neuronID uint64) uint16 {
    byteOffset := neuronID*6 + 2
    return *(*uint16)(unsafe.Pointer(&n.data[byteOffset]))
}

// Получение type (4 бита, упакован с threshold и resting)
func (n *NeuronArray) GetType(neuronID uint64) uint8 {
    byteOffset := neuronID*6 + 4
    packed := n.data[byteOffset]
    return (packed >> 4) & 0x0F
}

// Получение threshold (4 бита, упакован)
func (n *NeuronArray) GetThreshold(neuronID uint64) uint8 {
    byteOffset := neuronID*6 + 4
    packed := n.data[byteOffset]
    return packed & 0x0F
}

// Получение resting_potential (8 бит)
func (n *NeuronArray) GetRestingPotential(neuronID uint64) uint8 {
    byteOffset := neuronID*6 + 5
    return n.data[byteOffset]
}
```

### Goroutines для параллельного доступа

```go
package memory

import "sync"

type MemorySystem struct {
    base       *BaseReader
    delta      *DeltaReader
    cache      *LRUCache
    mu         sync.RWMutex
    workerPool chan func()
}

func NewMemorySystem(basePath, deltaPath string, workers int) *MemorySystem {
    ms := &MemorySystem{
        workerPool: make(chan func(), workers*2),
    }

    // Запускаем worker pool
    for i := 0; i < workers; i++ {
        go ms.worker()
    }

    return ms
}

func (ms *MemorySystem) worker() {
    for fn := range ms.workerPool {
        fn()
    }
}

// Параллельное чтение синапсов для нескольких нейронов
func (ms *MemorySystem) GetSynapsesBatch(neuronIDs []uint32) map[uint32][]Synapse {
    results := make(map[uint32][]Synapse)
    var wg sync.WaitGroup
    var mu sync.Mutex

    for _, id := range neuronIDs {
        wg.Add(1)
        ms.workerPool <- func() {
            defer wg.Done()
            synapses := ms.GetSynapses(id)

            mu.Lock()
            results[id] = synapses
            mu.Unlock()
        }
    }

    wg.Wait()
    return results
}
```

### LZ4 через cgo

```go
package lz4

/*
#cgo LDFLAGS: -llz4
#include <lz4.h>
*/
import "C"
import "unsafe"

func Compress(src []byte) []byte {
    maxDstSize := C.LZ4_compressBound(C.int(len(src)))
    dst := make([]byte, maxDstSize)

    compressedSize := C.LZ4_compress_default(
        (*C.char)(unsafe.Pointer(&src[0])),
        (*C.char)(unsafe.Pointer(&dst[0])),
        C.int(len(src)),
        maxDstSize,
    )

    return dst[:compressedSize]
}

func Decompress(src []byte, dstSize int) []byte {
    dst := make([]byte, dstSize)

    decompressedSize := C.LZ4_decompress_safe(
        (*C.char)(unsafe.Pointer(&src[0])),
        (*C.char)(unsafe.Pointer(&dst[0])),
        C.int(len(src)),
        C.int(dstSize),
    )

    if decompressedSize < 0 {
        return nil // Ошибка декомпрессии
    }

    return dst[:decompressedSize]
}
```

### Channels для event-driven архитектуры

```go
package memory

type Event struct {
    Type    EventType
    Payload interface{}
}

type MemorySystem struct {
    eventChan chan Event
    done      chan struct{}
}

func (ms *MemorySystem) StartEventLoop() {
    go func() {
        for {
            select {
            case event := <-ms.eventChan:
                ms.handleEvent(event)
            case <-ms.done:
                return
            }
        }
    }()
}

func (ms *MemorySystem) handleEvent(event Event) {
    switch event.Type {
    case SpikeEvent:
        spike := event.Payload.(Spike)
        ms.processSpike(spike)
    case STDPEvent:
        stdp := event.Payload.(STDPUpdate)
        ms.updateWeight(stdp)
    }
}
```

---

## 💻 Примеры кода

### Инициализация системы памяти

```go
package main

import (
    "log"
    "runtime"

    "github.com/Major-Woolfi/Project_Mira/libs/go/internal/memory"
)

func main() {
    // Устанавливаем количество потоков = количество ядер CPU
    runtime.GOMAXPROCS(runtime.NumCPU())

    // Инициализируем систему памяти
    ms := memory.NewMemorySystem(
        "data/memory/base.mcog",
        "data/memory/delta.wal",
        runtime.NumCPU(), // Worker pool
    )

    // Открываем файлы
    if err := ms.Open(); err != nil {
        log.Fatal(err)
    }
    defer ms.Close()

    // Запускаем event loop
    ms.StartEventLoop()

    // Запускаем compaction в фоне
    go ms.RunCompactionLoop()

    // Основной цикл работы Миры
    for {
        // Читаем синапсы для активных нейронов
        activeNeurons := ms.GetActiveNeurons()
        synapses := ms.GetSynapsesBatch(activeNeurons)

        // Обрабатываем спайки
        spikes := ms.SimulateStep(synapses)

        // Применяем STDP
        ms.ApplySTDP(spikes)

        // Логируем статистику
        ms.LogStats()
    }
}
```

### Чтение и декодирование нейрона

```go
package memory

import (
    "fmt"
    "unsafe"
)

type Neuron struct {
    ID               uint32
    Seed             uint16
    ClusterID        uint16
    Type             uint8
    Threshold        float32 // Декодировано из uint8
    RestingPotential float32 // Декодировано из uint8
}

func (ms *MemorySystem) GetNeuron(id uint32) Neuron {
    // Читаем из mmap
    seed := ms.base.GetSeed(id)
    clusterID := ms.base.GetClusterID(id)
    typePacked := ms.base.GetType(id)
    thresholdPacked := ms.base.GetThreshold(id)
    restingPacked := ms.base.GetRestingPotential(id)

    // Декодируем квантованные значения
    threshold := decodeThreshold(thresholdPacked)
    resting := decodeRestingPotential(restingPacked)

    return Neuron{
        ID:               id,
        Seed:             seed,
        ClusterID:        clusterID,
        Type:             typePacked,
        Threshold:        threshold,
        RestingPotential: resting,
    }
}

func decodeThreshold(quantized uint8) float32 {
    // Диапазон: -60 до -40 мВ
    return -60.0 + float32(quantized)*(20.0/256.0)
}

func decodeRestingPotential(quantized uint8) float32 {
    // Диапазон: -75 до -55 мВ
    return -75.0 + float32(quantized)*(20.0/256.0)
}
```

### Симуляция одного тика

```go
package memory

import "sync"

func (ms *MemorySystem) SimulateStep(synapses map[uint32][]Synapse) []Spike {
    var spikes []Spike
    var mu sync.Mutex
    var wg sync.WaitGroup

    // Параллельно обрабатываем каждый нейрон
    for neuronID, neuronSynapses := range synapses {
        wg.Add(1)
        go func(id uint32, syns []Synapse) {
            defer wg.Done()

            // Получаем текущее состояние нейрона
            V, w := ms.GetNeuronState(id)

            // Вычисляем входной ток
            I := ms.computeInputCurrent(id, syns)

            // Интегрируем AdEx (один шаг Эйлера, dt = 1 мс)
            V_new, w_new, spiked := ms.integrateAdEx(V, w, I, 1.0)

            // Обновляем состояние
            ms.SetNeuronState(id, V_new, w_new)

            // Если был спайк, добавляем в список
            if spiked {
                mu.Lock()
                spikes = append(spikes, Spike{
                    NeuronID:  id,
                    Timestamp: ms.currentTime,
                })
                mu.Unlock()
            }
        }(neuronID, neuronSynapses)
    }

    wg.Wait()
    return spikes
}

func (ms *MemorySystem) integrateAdEx(V, w, I, dt float32) (float32, float32, bool) {
    // Параметры AdEx
    const (
        C      = 281.0  // пФ
        gL     = 30.0   // нС
        EL     = -70.6   // мВ
        VT     = -50.4   // мВ
        deltaT = 2.0    // мВ
        tauW   = 144.0  // мс
        a      = 4.0    // нС
        b      = 0.0805 // нА
    )

    neuron := ms.GetNeuron(neuronID)
    threshold := neuron.Threshold

    // Уравнение для V
    dV := (-gL*(V-EL) + gL*deltaT*float32(math.Exp(float64((V-VT)/deltaT))) - w + I) / C * dt

    // Уравнение для w
    dw := (a*(V-EL) - w) / tauW * dt

    V_new := V + dV
    w_new := w + dw

    // Проверка спайка
    if V_new > threshold {
        // Сброс после спайка
        V_new = neuron.RestingPotential
        w_new += b
        return V_new, w_new, true
    }

    return V_new, w_new, false
}
```

---

## 🚀 Оптимизации

### 1. Cache-friendly доступ (SoA)

Вместо AoS (Array of Structures):

```go
// ПЛОХО: каждый нейрон = отдельная структура
type NeuronAoS struct {
    Seed      uint16
    ClusterID uint16
    Type      uint8
    // ...
}
neurons := []NeuronAoS{...}
```

Используем SoA (Structure of Arrays):

```go
// ХОРОШО: все seeds подряд, все clusterIDs подряд
type NeuronSoA struct {
    Seeds      []uint16
    ClusterIDs []uint16
    Types      []uint8
    // ...
}
```

**Преимущество:** При обновлении всех потенциалов нейронов CPU загружает в кэш только массив потенциалов, а не всю структуру нейрона. Это даёт **10-100× ускорение** для векторизованных операций.

### 2. Prefetching через mmap

```go
import "syscall"

// Предзагрузка страниц в RAM
func (m *MMap) Prefetch(offset, size int) error {
    _, _, errno := syscall.Syscall(
        syscall.SYS_MADVISE,
        uintptr(unsafe.Pointer(&m.data[offset])),
        uintptr(size),
        uintptr(syscall.MADV_WILLNEED),
    )
    if errno != 0 {
        return errno
    }
    return nil
}
```

**Когда использовать:** Перед обращением к "холодным" зонам коры (которые давно не использовались).

### 3. SIMD через AVX2

```go
//go:noescape
func updatePotentialsAVX2(potentials []float32, currents []float32, dt float32)

// Реализация на ассемблере (plan9 syntax)
TEXT ·updatePotentialsAVX2(SB), NOSPLIT, $0
    MOVQ potentials+0(FP), SI
    MOVQ currents+24(FP), DX
    MOVQ dt+48(FP), X0

    // Загружаем 8 потенциалов в YMM0
    VMOVUPS (SI), Y0

    // Загружаем 8 токов в YMM1
    VMOVUPS (DX), Y1

    // V = V + I * dt
    VMULPS Y1, X0, Y1
    VADDPS Y1, Y0, Y0

    // Сохраняем обратно
    VMOVUPS Y0, (SI)
    RET
```

**Ускорение:** 8× для операций над массивами.

### 4. Zero-copy чтение через unsafe

```go
// Вместо копирования данных, возвращаем указатель на mmap
func (ms *MemorySystem) GetSynapsesUnsafe(neuronID uint32) []Synapse {
    offset := ms.base.GetSynapseOffset(neuronID)
    count := ms.base.GetSynapseCount(neuronID)

    // Получаем указатель на данные в mmap
    ptr := unsafe.Pointer(&ms.base.data[offset])

    // Создаём срез без копирования
    return (*[1 << 20]Synapse)(ptr)[:count:count]
}
```

**Важно:** Срез действителен только пока открыт mmap!

### 5. Batch-обработка спайков

```go
// Вместо обработки каждого спайка отдельно, собираем их в batch
type SpikeBatch struct {
    NeuronIDs []uint32
    Timestamp uint64
}

func (ms *MemorySystem) ProcessSpikeBatch(batch SpikeBatch) {
    // Параллельно обновляем веса для всех спайков
    var wg sync.WaitGroup

    for _, id := range batch.NeuronIDs {
        wg.Add(1)
        go func(neuronID uint32) {
            defer wg.Done()
            ms.applySTDPForNeuron(neuronID, batch.Timestamp)
        }(id)
    }

    wg.Wait()
}
```

**Ускорение:** Уменьшает overhead на запуск goroutines.

---

## ⚖️ Компромиссы

### Что мы теряем

1. **Нельзя быстро перебрать все синапсы нейрона**
   - Приходится генерировать процедурно + фильтровать Delta
   - Медленнее, чем явный CSR
   - **Митигация:** Кэшируем процедурные синапсы в LRU

2. **Сложнее обновлять веса**
   - Нужно проверять Delta перед записью
   - **Митигация:** RAM-индекс последних записей Delta

3. **Невозможно точно воспроизвести биологию 1 в 1**
   - Процедурные правила — аппроксимация генетических механизмов
   - **Митигация:** Используем биологически обоснованные вероятности (из литературы)

4. **Compaction может занимать время**
   - При большом Delta слияние занимает минуты
   - **Митигация:** Запускаем в фоне, когда Мира "спит"

### Что мы получаем

1. **Человеческий масштаб на одном SSD**
   - 86 млрд нейронов в 512 ГБ — физически невозможно с явным CSR

2. **Быстрый старт**
   - Загрузка параметров нейронов (387 ГБ) через mmap за ~1 секунду

3. **Эффективное хранение**
   - Тратим память только на то, что реально меняется (обучение)

4. **Динамический нейрогенез**
   - Новые нейроны добавляются без пересборки файлов

---

## 💾 RAM-стратегия

### Hot Set (горячие данные)

В RAM всегда держим:

1. **CSR-указатели** (`row_ptr`) — 8 ГБ для 1 млрд нейронов
2. **Активные состояния нейронов** (V, w) — 3-5 ГБ (только для 5% активных)
3. **Delta-индекс** — 2-4 ГБ (последние 1000 записей)
4. **Procedural Cache** — 4-6 ГБ (LRU для сгенерированных синапсов)

**Итого:** ~17-23 ГБ постоянно в RAM

### Warm Set (тёплые данные)

Подгружаются через mmap + OS page cache:

1. **Base параметры нейронов** — подгружаются по мере обращения
2. **Delta-записи** — декомпрессируются LZ4 при чтении
3. **Connectivity Rules** — кэшируются при первом использовании

**Итого:** ~10-15 ГБ в page cache (OS сама управляет)

### Cold Set (холодные данные)

Остаются на SSD, подгружаются только при обращении:

1. **Неиспользуемые зоны коры** — выгружаются OS при нехватке RAM
2. **Старые Delta-записи** — ждут compaction
3. **Процедурные синапсы** — генерируются заново при обращении (экономия RAM)

### Стратегия вытеснения (Eviction Policy)

Используем **3-уровневую LRU-стратегию**:

```go
package memory

type LRUCache struct {
    capacity int
    items    map[uint32]*list.Element
    order    *list.List
    mu       sync.Mutex
}

type CacheItem struct {
    Key       uint32
    Synapses  []Synapse
    Size      int
    LastUsed  time.Time
    AccessCnt uint32
}

// Eviction: вытесняем сначала редко используемое, потом старое
func (c *LRUCache) Evict(bytesNeeded int) {
    c.mu.Lock()
    defer c.mu.Unlock()

    freed := 0
    for freed < bytesNeeded && c.order.Len() > 0 {
        // Берём элемент из хвоста (самый старый)
        elem := c.order.Back()
        item := elem.Value.(*CacheItem)

        // Не вытесняем "горячие" элементы (много обращений)
        if item.AccessCnt > 100 && time.Since(item.LastUsed) < time.Minute {
            // Перемещаем в начало, пропускаем
            c.order.MoveToFront(elem)
            continue
        }

        freed += item.Size
        delete(c.items, item.Key)
        c.order.Remove(elem)
    }
}
```

**Приоритет вытеснения:**

1. 🔴 **Холодное и старое** — вытесняется первым
2. 🟡 **Холодное, но частое** — вытесняется вторым
3. 🟢 **Горячее** — остаётся в RAM

---

## 📊 Мониторинг и отладка

### Метрики (экспорт в Prometheus)

```go
package memory

import "github.com/prometheus/client_golang/prometheus"

var (
    neuronsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "mira_neurons_total",
        Help: "Общее количество нейронов в системе",
    })

    synapsesGenerated = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "mira_synapses_generated_total",
        Help: "Количество процедурно сгенерированных синапсов",
    })

    deltaRecordsWritten = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "mira_delta_records_written_total",
        Help: "Количество записей в Delta-лог",
    })

    cacheHitRatio = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "mira_cache_hit_ratio",
        Help: "Процент попаданий в procedural cache",
    })

    mmapPageFaults = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "mira_mmap_page_faults_total",
        Help: "Количество page faults при mmap-доступе",
    })

    compactionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "mira_compaction_duration_seconds",
        Help:    "Время выполнения compaction",
        Buckets: prometheus.ExponentialBuckets(1, 2, 10),
    })
)
```

### Админ-панель: эндпоинты

```go
// GET /api/memory/stats
func (ms *MemorySystem) StatsHandler(w http.ResponseWriter, r *http.Request) {
    stats := map[string]interface{}{
        "neurons_total":      ms.base.TotalNeurons,
        "clusters_total":     ms.base.TotalClusters,
        "delta_records":      ms.delta.RecordCount(),
        "delta_size_mb":      ms.delta.FileSize() / (1024 * 1024),
        "cache_size_mb":      ms.cache.MemoryUsage() / (1024 * 1024),
        "cache_hit_ratio":    ms.cache.HitRatio(),
        "last_compaction":    ms.lastCompactionTime,
        "mmap_resident_mb":   ms.getResidentMemoryMB(),
        "active_neurons_pct": ms.getActiveNeuronsPercent(),
    }
    json.NewEncoder(w).Encode(stats)
}

// GET /api/memory/neuron/{id}
func (ms *MemorySystem) NeuronHandler(w http.ResponseWriter, r *http.Request) {
    id := parseIDFromURL(r)
    neuron := ms.GetNeuron(id)
    synapses := ms.GetSynapses(id)

    response := map[string]interface{}{
        "neuron":         neuron,
        "synapses_count": len(synapses),
        "synapses":       synapses[:min(100, len(synapses))], // Первые 100
        "procedural":     ms.countProceduralSynapses(id),
        "delta_modified": ms.countDeltaModifications(id),
    }
    json.NewEncoder(w).Encode(response)
}

// POST /api/memory/compact
func (ms *MemorySystem) CompactHandler(w http.ResponseWriter, r *http.Request) {
    go ms.RunCompaction() // Запускаем в фоне
    w.WriteHeader(http.StatusAccepted)
    w.Write([]byte("Compaction started"))
}
```

### Профилирование через pprof

```go
import _ "net/http/pprof"

func main() {
    // Запускаем pprof-сервер на :6060
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()

    // Теперь доступно:
    // http://localhost:6060/debug/pprof/heap     — использование памяти
    // http://localhost:6060/debug/pprof/profile  — CPU профиль
    // http://localhost:6060/debug/pprof/goroutine — горутины
    // http://localhost:6060/debug/pprof/block    — блокировки
}
```

**Полезные команды:**

```bash
# Анализ использования памяти
go tool pprof http://localhost:6060/debug/pprof/heap

# Анализ CPU (30 секунд)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Трассировка (для отладки race conditions)
go tool trace http://localhost:6060/debug/pprof/trace?seconds=5
```

### Race detector

```bash
# Запуск с race detector (для выявления data races)
go run -race ./cmd/mira-memory

# Тесты с race detector
go test -race ./internal/memory/...
```

---

## 🔄 Версионирование формата

### Схема версионирования

```go
type FormatVersion struct {
    Major uint16  // Несовместимые изменения (ребилд базы)
    Minor uint16  // Новые возможности (обратная совместимость)
    Patch uint16  // Баг-фиксы
}

const CurrentVersion = FormatVersion{1, 0, 0}
```

### Миграции

```go
package migrations

type Migration interface {
    From() FormatVersion
    To() FormatVersion
    Migrate(basePath, deltaPath string) error
}

type Migrator struct {
    migrations []Migration
}

func (m *Migrator) Register(mig Migration) {
    m.migrations = append(m.migrations, mig)
}

func (m *Migrator) MigrateIfNeeded(basePath string) error {
    header := readHeader(basePath)
    current := FormatVersion{header.Major, header.Minor, header.Patch}
    target := CurrentVersion

    if current == target {
        return nil // Ничего делать не нужно
    }

    // Применяем миграции по цепочке
    for _, mig := range m.migrations {
        if mig.From() == current {
            if err := mig.Migrate(basePath, deltaPath); err != nil {
                return err
            }
            current = mig.To()
            if current == target {
                break
            }
        }
    }

    return nil
}
```

### История версий

| Версия | Дата    | Изменения                                   |
| ------ | ------- | ------------------------------------------- |
| 0.1.0  | 2026-08 | Первая версия: простой CSR в .bin           |
| 0.2.0  | 2026-09 | Добавлена таблица разделов                  |
| 0.3.0  | 2026-10 | SoA формат, квантование параметров          |
| 1.0.0  | 2026-12 | Процедурная связность + Delta WAL (текущая) |

---

## 📋 Чек-лист внедрения

### Этап 1: Базовая структура (v0.2)

- [ ] Создать `base.mcog` с Header и Codebook
- [ ] Реализовать mmap-обёртку на Go
- [ ] Написать bit-packing для NeuronParams
- [ ] Интегрировать PCG32 PRNG

### Этап 2: Процедурная связность (v0.3)

- [ ] Определить Connectivity Rules (биологические вероятности)
- [ ] Реализовать Cluster Metadata
- [ ] Написать `generateProceduralSynapses()`
- [ ] Создать unit-тесты для детерминированности PRNG

### Этап 3: Delta-лог (v0.4)

- [ ] Реализовать Append-Only WAL
- [ ] Интегрировать LZ4 через cgo
- [ ] Написать буферизованную запись (64 КБ блоки)
- [ ] Создать RAM-индекс для быстрого поиска

### Этап 4: Интеграция (v0.5)

- [ ] Связать Go-модуль с Python через cgo (или FFI)
- [ ] Реализовать `GetSynapses()` с объединением Base + Delta
- [ ] Написать фоновый compaction
- [ ] Интегрировать с Core AI

### Этап 5: Оптимизация (v0.6+)

- [ ] Добавить LRU-кэш для процедурных синапсов
- [ ] Реализовать SIMD (AVX2) для AdEx-интеграции
- [ ] Настроить pprof и Prometheus-метрики
- [ ] Провести нагрузочное тестирование

---

## 🎯 Заключение

Архитектура памяти проекта "Мира" v3 — это **прорыв в хранении биологических нейросетей**. Мы смогли:

✅ **Уместить 86 млрд нейронов и 130 трлн синапсов в 512 ГБ SSD**
✅ **Работать в 64 ГБ RAM** благодаря mmap + OS page cache
✅ **Поддерживать динамический нейрогенез** без пересборки файлов
✅ **Использовать Go** для производительности и безопасности памяти
✅ **Сохранить биологическую правдоподобность** через AdEx и процедурные правила

Ключевая инновация — разделение на **природу (Nature)** и **опыт (Nurture)**:

- **Nature** хранится как детерминированные правила (seed + PRNG)
- **Nurture** хранится как sparse modifications в Delta-логе

Это даёт беспрецедентную эффективность: мы тратим память только на то, что действительно меняется в процессе обучения, а не на всю структуру мозга целиком.

---

## 📚 Биологические референсы

1. **Herculano-Houzel, S. (2009).** "The human brain in numbers: a linearly scaled-up primate brain"
   - 86 миллиардов нейронов в человеческом мозге
   - Распределение по регионам (кора, мозжечок, подкорка)

2. **Brette, R., & Gerstner, W. (2005).** "Adaptive Exponential Integrate-and-Fire Model as an Effective Description of Neuronal Activity"
   - Оригинальная статья по модели AdEx
   - Параметры для разных типов нейронов

3. **Markram, H., et al. (2015).** "Reconstruction and Simulation of Neocortical Microcircuitry"
   - Blue Brain Project: вероятности связности между типами нейронов
   - Топология кортикальных колонок

4. **Song, S., et al. (2005).** "Highly nonrandom features of synaptic connectivity in local cortical circuits"
   - Логнормальное распределение весов синапсов
   - Кластеризация связей

5. **Izhikevich, E. M. (2003).** "Simple model of spiking neurons"
   - Сравнение различных моделей спайковых нейронов
   - Классификация firing patterns

---

## 🔗 Полезные ссылки

- **PCG32 PRNG:** https://www.pcg-random.org/
- **LZ4 compression:** https://lz4.github.io/lz4/
- **Go mmap package:** https://pkg.go.dev/golang.org/x/sys/unix#Mmap
- **Prometheus client_golang:** https://github.com/prometheus/client_golang
- **Blue Brain Project:** https://www.epfl.ch/research/domains/bluebrain/
