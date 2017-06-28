package pgsrv

import (
    "fmt"
    "encoding/binary"
)

var TypesOid = map[string]int{
    "BOOL": 16,
    "BYTEA": 17,
    "CHAR": 18,
    "INT8": 20,
    "INT2": 21,
    "INT4": 23,
    "INT": 20,
    "TEXT": 25,
    "JSON": 114,
    "XML": 142,
    "FLOAT4": 700,
    "FLOAT8": 701,
    "VARCHAR": 1043,
    "DATE": 1082,
    "TIME": 1083,
    "TIMESTAMP": 1114,
    "TIMESTAMPZ": 1184,
    "INTERVAL": 1186,
    "NUMERIC": 1700,
    "JSONB": 3802,
    "ANY": 2276,
}

// QueryText returns the SQL query string from a Query or Parse message
func (m msg) QueryText() (string, error) {
    if m.Type() != 'Q' {
        return "", fmt.Errorf("Not a query message: %q", m.Type())
    }

    return string(m[5:]), nil
}

// RowDescriptionMsg is a message indicating that DataRow messages are about to
// be transmitted and delivers their schema (column names/types)
func rowDescriptionMsg(cols, types []string) msg {
    msg := []byte{'T', /* LEN = */ 0, 0, 0, 0, /* NUM FIELDS = */ 0, 0}
    binary.BigEndian.PutUint16(msg[5:], uint16(len(cols)))

    for i, c := range cols {
        msg = append(msg, []byte(c)...)
        msg = append(msg, 0) // NULL TERMINATED

        msg = append(msg, 0, 0, 0, 0) // object ID of the table; otherwise zero
        msg = append(msg, 0, 0) // attribute number of the column; otherwise zero

        // object ID of the field's data type
        oid := []byte{0,0,0,0}
        binary.BigEndian.PutUint32(oid, uint32(TypesOid[types[i]]))
        msg = append(msg, oid...)
        msg = append(msg, 0, 0) // data type size
        msg = append(msg, 0, 0, 0, 0) // type modifier
        msg = append(msg, 0, 0) // format code (text = 0, binary = 1)
    }

    // write the length
    binary.BigEndian.PutUint32(msg[1:5], uint32(len(msg) - 1))
    return msg
}

func dataRowMsg(vals []string) msg {
    msg := []byte{'D', /* LEN = */ 0, 0, 0, 0, /* NUM VALS = */ 0, 0}
    binary.BigEndian.PutUint16(msg[5:], uint16(len(vals)))

    for _, v := range vals {
        b := append(make([]byte, 4), []byte(v)...)
        binary.BigEndian.PutUint32(b[0:4], uint32(len(b) - 4))
        msg = append(msg, b...)
    }

    // write the length
    binary.BigEndian.PutUint32(msg[1:5], uint32(len(msg) - 1))
    return msg
}

func completeMsg(tag string) msg {
    msg := []byte{'C', 0, 0, 0, 0}
    msg = append(msg, []byte(tag)...)
    msg = append(msg, 0) // NULL TERMINATED

    // write the length
    binary.BigEndian.PutUint32(msg[1:5], uint32(len(msg) - 1))
    return msg
}

func errMsg(err error) msg {
    msg := []byte{'E', 0, 0, 0, 0}

    // https://www.postgresql.org/docs/9.3/static/protocol-error-fields.html
    fields := map[string]string{
        "S": "ERROR",
        "C": "XX000",
        "M": err.Error(),
    }

    // error code
    errCode, ok := err.(interface { Code() string })
    if ok && errCode.Code() != "" {
        fields["C"] = errCode.Code()
    }

    // hint
    errHint, ok := err.(interface { Hint() string })
    if ok && errHint.Hint() != "" {
        fields["H"] = errHint.Hint()
    }

    // cursor position
    errLoc, ok := err.(interface { Loc() int })
    if ok && errLoc.Loc() >= 0 {
        fields["P"] = fmt.Sprintf("%d", errLoc.Loc())
    }

    for k, v := range fields {
        msg = append(msg, byte(k[0]))
        msg = append(msg, []byte(v)...)
        msg = append(msg, 0) // NULL TERMINATED
    }

    msg = append(msg, 0) // NULL TERMINATED

    // write the length
    binary.BigEndian.PutUint32(msg[1:5], uint32(len(msg) - 1))
    return msg
}

// ReadyMsg is sent whenever the backend is ready for a new query cycle.
func readyMsg() msg {
    return []byte{'Z', 0, 0, 0, 5, 'I'}
}
