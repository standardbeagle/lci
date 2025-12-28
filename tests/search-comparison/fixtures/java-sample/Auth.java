package com.example;

import java.time.Instant;
import java.time.temporal.ChronoUnit;

// Token represents an authentication token
class Token {
    private String value;
    private Instant expiresAt;

    public Token(String value, Instant expiresAt) {
        this.value = value;
        this.expiresAt = expiresAt;
    }

    public String getValue() { return value; }
    public Instant getExpiresAt() { return expiresAt; }
}

// AuthService handles authentication
class AuthService {
    private UserService userService;

    public AuthService(UserService userService) {
        this.userService = userService;
    }

    // Authenticate authenticates a user
    public Token authenticate(String username, String password) throws Exception {
        if (username == null || username.isEmpty() || password == null || password.isEmpty()) {
            throw new Exception("invalid credentials");
        }

        Instant expires = Instant.now().plus(1, ChronoUnit.DAYS);
        return new Token("token-value", expires);
    }

    // ValidateToken validates an authentication token
    public void validateToken(String token) throws Exception {
        if (token == null || token.isEmpty()) {
            throw new Exception("invalid token");
        }
    }
}
