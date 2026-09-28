package model

type DataType string

// 数据类型

const (
	Bool    DataType = "bool"
	Int16   DataType = "int16"
	UInt16  DataType = "uint16"
	Int32   DataType = "int32"
	UInt32  DataType = "uint32"
	Float32 DataType = "float32"
	Float64 DataType = "float64"
)

// Modbus寄存器类型

type RegisterType string

const (
	Coil            RegisterType = "coil"             // 线圈
	DiscreteInput   RegisterType = "discrete_input"   // 离散输入
	HoldingRegister RegisterType = "holding_register" // 保持寄存器
	InputRegister   RegisterType = "input_register"   // 输入寄存器
)

type ByteOrder string

const (
	ABCD ByteOrder = "ABCD" // 默认大端
	BADC ByteOrder = "BADC" // 交换每个寄存器内两字节
	CDAB ByteOrder = "CDAB" // 高低寄存器互换
	DCBA ByteOrder = "DCBA" // 全部反转
)

type TagConfig struct {
	Name         string       `json:"name"`                  // 点位名称
	Address      string       `json:"address"`               // modbus地址字符串，支持5位IEC格式如40001
	RegisterType RegisterType `json:"register_type"`         // 寄存器类型
	DataType     DataType     `json:"data_type"`             // 解析数据类型
	SlaveID      byte         `json:"slave_id"`              // 从站ID
	Writable     bool         `json:"writable"`              // 是否可写
	Scale        float64      `json:"scale"`                 // 量程系数
	Offset       float64      `json:"offset"`                // 偏移量
	ByteOrder    ByteOrder    `json:"byte_order"`            // 字节序
	Description  string       `json:"description,omitempty"` // 点位说明
	History      bool         `json:"history"`               // 是否写入历史数据库
}

type DeviceConfig struct {
	Name                string      `json:"name"`
	Address             string      `json:"address"`               // ip:port
	TimeoutMS           int         `json:"timeout_ms"`            // 读写超时毫秒
	ReconnectIntervalMS int         `json:"reconnect_interval_ms"` // 重连间隔
	BatchGap            uint16      `json:"batch_gap"`             // 批量合并允许的寄存器间隙
	RegisterBatchSize   uint16      `json:"register_batch_size"`   // 寄存器单次最大读取数量
	BitBatchSize        uint16      `json:"bit_batch_size"`        // 线圈/离散输入单次最大读取数量
	Tags                []TagConfig `json:"tags"`
}

type AlarmRule struct {
	Tag        string   `json:"tag"`
	Enabled    bool     `json:"enabled"`
	DataType   DataType `json:"data_type"`
	AlarmValue *bool    `json:"alarm_value,omitempty"`
	Low        *float64 `json:"low"`
	High       *float64 `json:"high"`
	Unit       string   `json:"unit,omitempty"`
	Message    string   `json:"message,omitempty"`
}

type AlarmConfig struct {
	Enabled bool        `json:"enabled"`
	Alarms  []AlarmRule `json:"alarms"`
}

type HistoryConfig struct {
	Enabled               bool   `json:"enabled"`
	SampleIntervalMinutes int    `json:"sample_interval_minutes"`
	RetentionDays         int    `json:"retention_days"`
	ArchiveEnabled        bool   `json:"archive_enabled"`
	ArchiveDir            string `json:"archive_dir"`
	ArchiveRetentionDays  int    `json:"archive_retention_days"`
	CleanupIntervalHours  int    `json:"cleanup_interval_hours"`
	QueryMaxPoints        int    `json:"query_max_points"`
}

type Config struct {
	PollIntervalMS   int                 `json:"poll_interval_ms"`
	ModbusEnabled    bool                `json:"enabled"`
	Devices          []DeviceConfig      `json:"devices"`
	OPCDAEnabled     bool                `json:"opcda_enabled"`
	OPCDADevices     []OPCDADeviceConfig `json:"opcda_devices"`
	ModbusEnabledSet bool                `json:"-"`
	OPCDAEnabledSet  bool                `json:"-"`
	Alarm            AlarmConfig         `json:"alarm"`
	History          HistoryConfig       `json:"history"`
}

func (c Config) IsModbusEnabled() bool {
	if c.ModbusEnabledSet {
		return c.ModbusEnabled
	}
	return len(c.Devices) > 0
}

func (c Config) IsOPCDAEnabled() bool {
	if c.OPCDAEnabledSet {
		return c.OPCDAEnabled
	}
	return len(c.OPCDADevices) > 0
}

// OPC DA classic COM/DCOM configuration. OPC DA uses a ProgID and Windows node;
// it does not use an opc.tcp endpoint like OPC UA.
type OPCDATagConfig struct {
	Name        string   `json:"name"`
	ItemID      string   `json:"item_id"`
	DataType    DataType `json:"data_type"`
	Writable    bool     `json:"writable"`
	Scale       float64  `json:"scale"`
	Offset      float64  `json:"offset"`
	Description string   `json:"description,omitempty"`
	History     bool     `json:"history"`
}

type OPCDADeviceConfig struct {
	Name                string           `json:"name"`
	ProgID              string           `json:"prog_id"`
	Node                string           `json:"node"`
	TimeoutMS           int              `json:"timeout_ms"`
	ReconnectIntervalMS int              `json:"reconnect_interval_ms"`
	GroupName           string           `json:"group_name"`
	Tags                []OPCDATagConfig `json:"tags"`
}
