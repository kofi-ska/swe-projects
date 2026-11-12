package com.powerliq.financial;

public enum PositionSide {
    LONG(1),
    SHORT(-1);

    private final int sign;

    PositionSide(int sign) {
        this.sign = sign;
    }

    public int sign() {
        return sign;
    }
}
