# Sample Python file for testing

def public_function(param):
    """Public function with docstring"""
    print(param)
    return param.upper()

def _private_function():
    """Private function (by convention)"""
    return 'private'

class PublicClass:
    """Public class with docstring"""

    def __init__(self, name):
        self.name = name
        self._private_attr = None

    def public_method(self):
        """Public method"""
        return self.name

    def _private_method(self):
        """Private method (by convention)"""
        return self._private_attr

class _PrivateClass:
    """Private class (by convention)"""

    def __init__(self):
        self.value = 0

# Public constant
PUBLIC_CONST = 'public'

# Private constant (by convention)
_PRIVATE_CONST = 'private'

# Module-level variable
global_state = None
