const std = @import("std");
const print = std.debug.print;

// Phase 5C: Basic Zig constructs

// Constants and variables
const PI: f32 = 3.14159;
var global_counter: u32 = 0;

// Struct definition
const Point = struct {
    x: f32,
    y: f32,
    
    // Method
    pub fn distance(self: Point, other: Point) f32 {
        const dx = self.x - other.x;
        const dy = self.y - other.y;
        return @sqrt(dx * dx + dy * dy);
    }
    
    // Associated function
    pub fn zero() Point {
        return Point{ .x = 0, .y = 0 };
    }
};

// Union
const Value = union(enum) {
    int: i32,
    float: f32,
    boolean: bool,
    
    pub fn asFloat(self: Value) f32 {
        return switch (self) {
            .int => |i| @intToFloat(f32, i),
            .float => |f| f,
            .boolean => |b| if (b) 1.0 else 0.0,
        };
    }
};

// Enum
const Color = enum {
    red,
    green,
    blue,
    
    pub fn toRgb(self: Color) [3]u8 {
        return switch (self) {
            .red => [3]u8{ 255, 0, 0 },
            .green => [3]u8{ 0, 255, 0 },
            .blue => [3]u8{ 0, 0, 255 },
        };
    }
};

// Function with error handling
fn divide(a: f32, b: f32) !f32 {
    if (b == 0) {
        return error.DivisionByZero;
    }
    return a / b;
}

// Generic function
fn swap(comptime T: type, a: *T, b: *T) void {
    const temp = a.*;
    a.* = b.*;
    b.* = temp;
}

// Function with optional return
fn findMax(numbers: []const i32) ?i32 {
    if (numbers.len == 0) return null;
    
    var max = numbers[0];
    for (numbers[1..]) |num| {
        if (num > max) max = num;
    }
    return max;
}

// Comptime function
fn fibonacci(comptime n: u32) u32 {
    if (n <= 1) return n;
    return fibonacci(n - 1) + fibonacci(n - 2);
}

// Main function
pub fn main() !void {
    print("Hello, Zig!\n");
    
    // Using structs
    var p1 = Point{ .x = 3.0, .y = 4.0 };
    const p2 = Point.zero();
    const dist = p1.distance(p2);
    print("Distance: {d}\n", .{dist});
    
    // Using unions
    const val = Value{ .int = 42 };
    print("Value as float: {d}\n", .{val.asFloat()});
    
    // Using enums
    const color = Color.red;
    const rgb = color.toRgb();
    print("Red RGB: {d}, {d}, {d}\n", .{ rgb[0], rgb[1], rgb[2] });
    
    // Error handling
    const result = divide(10.0, 2.0) catch |err| {
        print("Error: {}\n", .{err});
        return;
    };
    print("Division result: {d}\n", .{result});
    
    // Optional handling
    const numbers = [_]i32{ 1, 5, 3, 9, 2 };
    if (findMax(&numbers)) |max| {
        print("Max: {d}\n", .{max});
    } else {
        print("No max found\n");
    }
    
    // Comptime computation
    const fib_10 = comptime fibonacci(10);
    print("Fibonacci(10): {d}\n", .{fib_10});
    
    // Generic function usage
    var x: i32 = 10;
    var y: i32 = 20;
    swap(i32, &x, &y);
    print("After swap: x={d}, y={d}\n", .{ x, y });
}

// Test
test "point distance calculation" {
    const p1 = Point{ .x = 0, .y = 0 };
    const p2 = Point{ .x = 3, .y = 4 };
    const expected: f32 = 5.0;
    const actual = p1.distance(p2);
    try std.testing.expectEqual(expected, actual);
}

test "value conversion" {
    const int_val = Value{ .int = 42 };
    const float_val = Value{ .float = 3.14 };
    const bool_val = Value{ .boolean = true };
    
    try std.testing.expectEqual(@as(f32, 42.0), int_val.asFloat());
    try std.testing.expectEqual(@as(f32, 3.14), float_val.asFloat());
    try std.testing.expectEqual(@as(f32, 1.0), bool_val.asFloat());
}