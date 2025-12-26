<?php
declare(strict_types=1);

namespace App\Controllers;

use App\Services\UserService;

/**
 * PHP 8.0+ features test file
 */

// Class with attributes
#[Route('/api/users')]
#[Controller]
class UserController
{
    // Property with attribute
    #[Inject]
    private UserService $userService;

    // Constructor with property promotion
    public function __construct(
        private readonly string $name,
        protected int $id = 0,
        public ?string $email = null,
    ) {}

    // Method with multiple attributes
    #[Get('/')]
    #[Cache(ttl: 3600)]
    public function index(): array
    {
        return [];
    }

    // Method with single attribute and named arguments
    #[Route(path: '/users/{id}', methods: ['GET'])]
    public function show(int $id): array
    {
        return ['id' => $id];
    }
}

// Enum with attribute (PHP 8.1+)
#[Entity]
enum UserStatus: string
{
    case ACTIVE = 'active';
    case INACTIVE = 'inactive';

    public function label(): string
    {
        return match($this) {
            self::ACTIVE => 'Active',
            self::INACTIVE => 'Inactive',
        };
    }
}

// Readonly class (PHP 8.2+)
#[DTO]
readonly class UserDTO
{
    public function __construct(
        public string $name,
        public string $email,
    ) {}
}
