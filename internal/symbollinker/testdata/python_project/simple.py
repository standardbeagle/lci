"""
A simple Python module for testing symbol extraction
"""

import os
import sys
from typing import Dict, List, Optional, Union, Protocol
from collections import defaultdict, namedtuple
from dataclasses import dataclass, field
from abc import ABC, abstractmethod
from enum import Enum, IntEnum
import asyncio
import json as json_module

# Relative imports
from .utils import helper_function
from ..common import base_class

# Global constants
GLOBAL_CONSTANT = "global_value"
_PRIVATE_CONSTANT = "private_value"
PUBLIC_CONSTANT: str = "typed_constant"

# Global variables
global_var = "initialized"
_private_var = 42
typed_var: int = 100

# Type aliases
StringDict = Dict[str, str]
OptionalString = Optional[str]


class SimpleClass:
    """A simple class for testing."""
    
    class_variable = "shared"
    _private_class_var = "private_shared"
    
    def __init__(self, name: str, value: int = 10):
        """Initialize the class."""
        self.name = name
        self.value = value
        self._private_attr = "private"
        self.__name_mangled = "mangled"
    
    def public_method(self, param1: str, param2: int = 5) -> str:
        """A public method."""
        local_var = "local"
        return self._private_method(local_var)
    
    def _private_method(self, input_str: str) -> str:
        """A private method."""
        return input_str.upper()
    
    @property
    def name_property(self) -> str:
        """Property getter."""
        return self._private_attr
    
    @name_property.setter
    def name_property(self, value: str) -> None:
        """Property setter."""
        self._private_attr = value
    
    @staticmethod
    def static_method(x: int, y: int) -> int:
        """A static method."""
        return x + y
    
    @classmethod
    def class_method(cls, value: str):
        """A class method."""
        return cls(value, 0)
    
    async def async_method(self) -> List[str]:
        """An async method."""
        await asyncio.sleep(0.1)
        return ["async", "result"]
    
    def __str__(self) -> str:
        return f"SimpleClass({self.name})"
    
    def __enter__(self):
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        pass


class AbstractBase(ABC):
    """Abstract base class."""
    
    @abstractmethod
    def must_implement(self) -> None:
        """Must be implemented by subclasses."""
        pass
    
    def concrete_method(self) -> str:
        """Concrete method with implementation."""
        return "base implementation"


class DerivedClass(AbstractBase, SimpleClass):
    """Class inheriting from multiple bases."""
    
    def __init__(self, name: str, extra: str):
        super().__init__(name)
        self.extra = extra
    
    def must_implement(self) -> None:
        """Implementation of abstract method."""
        print("Implemented")
    
    def public_method(self, param1: str, param2: int = 5) -> str:
        """Override parent method."""
        result = super().public_method(param1, param2)
        return f"{result}_{self.extra}"


class GenericClass(ABC):
    """A generic class using type hints."""
    
    def __init__(self, items: List[Union[str, int]]):
        self.items: List[Union[str, int]] = items
    
    def add_item(self, item: Union[str, int]) -> None:
        self.items.append(item)
    
    def get_items(self) -> List[Union[str, int]]:
        return self.items.copy()


# Protocol definition
class Drawable(Protocol):
    """Protocol for drawable objects."""
    
    def draw(self) -> None:
        """Draw the object."""
        ...


# Dataclass
@dataclass
class Person:
    """Person dataclass."""
    name: str
    age: int
    email: Optional[str] = None
    tags: List[str] = field(default_factory=list)
    
    def get_display_name(self) -> str:
        return f"{self.name} ({self.age})"


# Enum classes
class Status(Enum):
    """Status enumeration."""
    PENDING = "pending"
    ACTIVE = "active"
    INACTIVE = "inactive"


class Priority(IntEnum):
    """Priority integer enumeration."""
    LOW = 1
    MEDIUM = 2
    HIGH = 3


# Named tuple
Point = namedtuple('Point', ['x', 'y'])
CoordinatePoint = namedtuple('CoordinatePoint', ['x', 'y', 'z'], defaults=[0])


# Global functions
def global_function(param: str) -> str:
    """A global function."""
    local_var = "local"
    return f"processed_{param}_{local_var}"


def _private_function() -> None:
    """A private global function."""
    pass


async def async_global_function(delay: float) -> Dict[str, str]:
    """An async global function."""
    await asyncio.sleep(delay)
    return {"status": "complete"}


def generic_function(items: List[T]) -> Optional[T]:
    """Generic function with type variables."""
    return items[0] if items else None


def variadic_function(*args, **kwargs) -> tuple:
    """Function with variadic arguments."""
    return args, kwargs


# Lambda functions
simple_lambda = lambda x: x * 2
complex_lambda = lambda x, y=10: x + y if x > 0 else y


# Generator function
def number_generator(n: int):
    """Generator function."""
    for i in range(n):
        yield i * 2


# Decorator
def my_decorator(func):
    """A simple decorator."""
    def wrapper(*args, **kwargs):
        print(f"Calling {func.__name__}")
        return func(*args, **kwargs)
    return wrapper


@my_decorator
def decorated_function(x: int) -> int:
    """A decorated function."""
    return x * 3


# Context manager
class MyContextManager:
    """Custom context manager."""
    
    def __enter__(self):
        print("Entering context")
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        print("Exiting context")


# Exception classes
class CustomException(Exception):
    """Custom exception class."""
    
    def __init__(self, message: str, code: int = 0):
        super().__init__(message)
        self.code = code


class SpecificError(CustomException):
    """More specific error."""
    pass


# Module-level code
if __name__ == "__main__":
    # Main execution block
    instance = SimpleClass("test", 42)
    result = instance.public_method("hello")
    print(result)
    
    # Exception handling
    try:
        risky_operation()
    except CustomException as e:
        print(f"Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")
    finally:
        cleanup()


# Type checking block
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from .other_module import SomeClass
    from typing import TypeVar
    
    T = TypeVar('T')


# Conditional imports
try:
    import numpy as np
    HAS_NUMPY = True
except ImportError:
    np = None
    HAS_NUMPY = False