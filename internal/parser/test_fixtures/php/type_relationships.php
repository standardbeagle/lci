<?php
declare(strict_types=1);

namespace App\Models;

// Interface with extends
interface BaseRepositoryInterface
{
    public function find(int $id): ?object;
    public function save(object $entity): bool;
}

interface UserRepositoryInterface extends BaseRepositoryInterface
{
    public function findByEmail(string $email): ?User;
}

interface AuditableInterface
{
    public function getCreatedAt(): \DateTimeInterface;
    public function getUpdatedAt(): \DateTimeInterface;
}

// Abstract base class
abstract class BaseModel
{
    protected int $id;

    abstract public function validate(): bool;

    public function getId(): int
    {
        return $this->id;
    }
}

// Trait definition
trait TimestampTrait
{
    protected \DateTimeInterface $createdAt;
    protected \DateTimeInterface $updatedAt;

    public function getCreatedAt(): \DateTimeInterface
    {
        return $this->createdAt;
    }

    public function getUpdatedAt(): \DateTimeInterface
    {
        return $this->updatedAt;
    }
}

trait SoftDeleteTrait
{
    protected ?\DateTimeInterface $deletedAt = null;

    public function isDeleted(): bool
    {
        return $this->deletedAt !== null;
    }
}

// Class with extends and implements
class User extends BaseModel implements AuditableInterface
{
    use TimestampTrait;
    use SoftDeleteTrait;

    private string $email;
    private string $name;

    public function __construct(string $email, string $name)
    {
        $this->email = $email;
        $this->name = $name;
    }

    public function validate(): bool
    {
        return filter_var($this->email, FILTER_VALIDATE_EMAIL) !== false;
    }
}

// Class implementing multiple interfaces
class UserRepository implements UserRepositoryInterface, AuditableInterface
{
    use TimestampTrait;

    private array $users = [];

    public function find(int $id): ?User
    {
        return $this->users[$id] ?? null;
    }

    public function save(object $entity): bool
    {
        if (!$entity instanceof User) {
            return false;
        }
        $this->users[$entity->getId()] = $entity;
        return true;
    }

    public function findByEmail(string $email): ?User
    {
        foreach ($this->users as $user) {
            if ($user->getEmail() === $email) {
                return $user;
            }
        }
        return null;
    }
}

// Final class
final class AdminUser extends User
{
    private array $permissions = [];

    public function addPermission(string $permission): void
    {
        $this->permissions[] = $permission;
    }
}

// PHP 8.1+ enum
enum UserStatus: string
{
    case ACTIVE = 'active';
    case INACTIVE = 'inactive';
    case PENDING = 'pending';

    public function label(): string
    {
        return match($this) {
            self::ACTIVE => 'Active User',
            self::INACTIVE => 'Inactive User',
            self::PENDING => 'Pending Verification',
        };
    }
}
