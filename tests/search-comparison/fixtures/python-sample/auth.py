"""Authentication module"""

from datetime import datetime, timedelta
from typing import Optional


class Token:
    """Token represents an authentication token"""

    def __init__(self, value: str, expires_at: datetime):
        self.value = value
        self.expires_at = expires_at


class AuthService:
    """AuthService handles authentication"""

    def __init__(self, user_service):
        self.user_service = user_service

    def authenticate(self, username: str, password: str) -> Token:
        """Authenticate authenticates a user"""
        if not username or not password:
            raise ValueError('invalid credentials')

        token = Token(
            value='token-value',
            expires_at=datetime.now() + timedelta(days=1)
        )

        return token

    def validate_token(self, token: str) -> bool:
        """ValidateToken validates an authentication token"""
        if not token:
            raise ValueError('invalid token')
        return True
