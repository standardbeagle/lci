using System;
using System.Collections.Generic;
using Microsoft.Extensions.DependencyInjection;

namespace MyApp.Services
{
    // Phase 5A: Record types (C# 9.0+)
    public record UserDto(int Id, string Name, string? Email);
    
    // Traditional class
    public class UserService
    {
        private readonly ILogger<UserService> _logger;
        
        // Constructor
        public UserService(ILogger<UserService> logger)
        {
            _logger = logger;
        }
        
        // Properties
        public string DatabaseConnection { get; set; } = string.Empty;
        public int MaxRetries { get; init; } = 3;
        
        // Events
        public event EventHandler<UserEventArgs>? UserCreated;
        
        // Methods
        public async Task<UserDto?> GetUserAsync(int id)
        {
            return await GetUserFromDatabaseAsync(id);
        }
        
        // Private methods
        private async Task<UserDto?> GetUserFromDatabaseAsync(int id)
        {
            // Local function (C# 7.0+)
            static bool IsValidId(int userId) => userId > 0;
            
            if (!IsValidId(id))
                return null;
                
            // Pattern matching (C# 8.0+)
            var result = id switch
            {
                > 0 and < 1000 => await FetchFromCache(id),
                >= 1000 => await FetchFromDatabase(id),
                _ => null
            };
            
            return result;
        }
        
        // Operator overloading
        public static UserService operator +(UserService left, UserService right)
        {
            return new UserService(left._logger);
        }
    }
    
    // Interface
    public interface IUserRepository
    {
        Task<UserDto?> GetByIdAsync(int id);
        
        // Indexer in interface
        UserDto? this[int id] { get; }
    }
    
    // Struct
    public struct Point
    {
        public int X { get; init; }
        public int Y { get; init; }
        
        // Constructor
        public Point(int x, int y)
        {
            X = x;
            Y = y;
        }
    }
    
    // Enum
    public enum UserStatus
    {
        Active,
        Inactive,
        Suspended
    }
    
    // Delegate
    public delegate void UserEventHandler(UserDto user);
    
    // Extension methods
    public static class UserExtensions
    {
        public static bool IsActive(this UserDto user) => 
            user.Id > 0;
    }
}