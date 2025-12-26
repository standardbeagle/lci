package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCSharpModernFeatures tests the c sharp modern features.
func TestCSharpModernFeatures(t *testing.T) {
	parser := NewTreeSitterParser()

	t.Run("parse C# records", func(t *testing.T) {
		code := `namespace MyApp
{
    // C# 9.0 record
    public record Person(string FirstName, string LastName);
    
    // C# 10.0 record struct
    public record struct Point(double X, double Y)
    {
        public double Distance() => System.Math.Sqrt(X * X + Y * Y);
    }
    
    // Record with inheritance
    public record Employee(string FirstName, string LastName, int Id) : Person(FirstName, LastName);
    
    // Record with body
    public record Customer
    {
        public string Name { get; init; }
        public string Email { get; init; }
        
        public void SendEmail(string message)
        {
            // Send email implementation
        }
    }
}`

		_, symbols, _ := parser.ParseFile("test.cs", []byte(code))

		// Check for records
		foundPerson := false
		foundPoint := false
		foundEmployee := false
		foundCustomer := false
		foundDistance := false
		foundSendEmail := false

		for _, sym := range symbols {
			switch sym.Name {
			case "Person":
				foundPerson = true
			case "Point":
				foundPoint = true
			case "Employee":
				foundEmployee = true
			case "Customer":
				foundCustomer = true
			case "Distance":
				foundDistance = true
			case "SendEmail":
				foundSendEmail = true
			}
		}

		assert.True(t, foundPerson, "Should find Person record")
		assert.True(t, foundPoint, "Should find Point record struct")
		assert.True(t, foundEmployee, "Should find Employee record")
		assert.True(t, foundCustomer, "Should find Customer record")
		assert.True(t, foundDistance, "Should find Distance method in record")
		assert.True(t, foundSendEmail, "Should find SendEmail method in record")
	})

	t.Run("parse C# delegates and events", func(t *testing.T) {
		code := `namespace MyApp
{
    public delegate void ProcessHandler(string message);
    public delegate Task<T> AsyncFunc<T>(string input);
    
    public class EventExample
    {
        public event ProcessHandler OnProcess;
        public event EventHandler<string> OnComplete;
        
        private event Action<int> privateEvent;
    }
}`

		_, symbols, _ := parser.ParseFile("test.cs", []byte(code))

		foundProcessHandler := false
		foundAsyncFunc := false
		foundOnProcess := false
		foundOnComplete := false
		foundPrivateEvent := false

		for _, sym := range symbols {
			switch sym.Name {
			case "ProcessHandler":
				foundProcessHandler = true
			case "AsyncFunc":
				foundAsyncFunc = true
			case "OnProcess":
				foundOnProcess = true
			case "OnComplete":
				foundOnComplete = true
			case "privateEvent":
				foundPrivateEvent = true
			}
		}

		assert.True(t, foundProcessHandler, "Should find ProcessHandler delegate")
		assert.True(t, foundAsyncFunc, "Should find AsyncFunc delegate")
		assert.True(t, foundOnProcess, "Should find OnProcess event")
		assert.True(t, foundOnComplete, "Should find OnComplete event")
		assert.True(t, foundPrivateEvent, "Should find privateEvent")
	})

	t.Run("parse C# nullable reference types and pattern matching", func(t *testing.T) {
		code := `#nullable enable
namespace MyApp
{
    public class NullableExample
    {
        public string? NullableProperty { get; set; }
        public string NonNullableProperty { get; set; } = "";
        
        public void PatternMatchingExample(object obj)
        {
            // Switch expression (C# 8.0)
            var result = obj switch
            {
                string s => s.Length,
                int i => i,
                null => 0,
                _ => -1
            };
            
            // Property pattern (C# 8.0)
            if (obj is Person { FirstName: "John" })
            {
                // Handle John
            }
        }
        
        // Init-only properties (C# 9.0)
        public string InitOnlyProp { get; init; }
    }
}`

		_, symbols, _ := parser.ParseFile("test.cs", []byte(code))

		foundNullableProperty := false
		foundNonNullableProperty := false
		foundPatternMatchingExample := false
		foundInitOnlyProp := false

		for _, sym := range symbols {
			switch sym.Name {
			case "NullableProperty":
				foundNullableProperty = true
			case "NonNullableProperty":
				foundNonNullableProperty = true
			case "PatternMatchingExample":
				foundPatternMatchingExample = true
			case "InitOnlyProp":
				foundInitOnlyProp = true
			}
		}

		assert.True(t, foundNullableProperty, "Should find NullableProperty")
		assert.True(t, foundNonNullableProperty, "Should find NonNullableProperty")
		assert.True(t, foundPatternMatchingExample, "Should find PatternMatchingExample method")
		assert.True(t, foundInitOnlyProp, "Should find InitOnlyProp")
	})

	t.Run("parse C# top-level programs and global using", func(t *testing.T) {
		code := `global using System;
global using System.Threading.Tasks;

using MyApp.Services;

// Top-level program (C# 9.0)
var builder = WebApplication.CreateBuilder(args);
var app = builder.Build();

app.MapGet("/", () => "Hello World!");

app.Run();

// Can still have types in top-level programs
public class Startup
{
    public void Configure(IApplicationBuilder app)
    {
        // Configuration
    }
}

public record WeatherForecast(DateTime Date, int TemperatureC);`

		_, symbols, imports := parser.ParseFile("Program.cs", []byte(code))

		// Check for types even in top-level program
		foundStartup := false
		foundWeatherForecast := false
		foundConfigure := false

		for _, sym := range symbols {
			switch sym.Name {
			case "Startup":
				foundStartup = true
			case "Configure":
				foundConfigure = true
			case "WeatherForecast":
				foundWeatherForecast = true
			}
		}

		assert.True(t, foundStartup, "Should find Startup class in top-level program")
		assert.True(t, foundConfigure, "Should find Configure method")
		assert.True(t, foundWeatherForecast, "Should find WeatherForecast record")
		assert.True(t, len(imports) > 0, "Should find imports including global usings")
	})

	t.Run("parse C# tuples and advanced pattern matching", func(t *testing.T) {
		code := `namespace MyApp
{
    public class TupleExample
    {
        // Tuple return type
        public (string name, int age) GetPerson()
        {
            return ("John", 30);
        }
        
        // Named tuple elements
        public (string First, string Last) SplitName(string fullName)
        {
            var parts = fullName.Split(' ');
            return (parts[0], parts[1]);
        }
        
        // Tuple deconstruction
        public void ProcessPerson()
        {
            var (name, age) = GetPerson();
            
            // Tuple pattern matching
            var point = (x: 10, y: 20);
            var quadrant = point switch
            {
                (0, 0) => "origin",
                (var x, var y) when x > 0 && y > 0 => "quadrant 1",
                (var x, var y) when x < 0 && y > 0 => "quadrant 2",
                (var x, var y) when x < 0 && y < 0 => "quadrant 3",
                (var x, var y) when x > 0 && y < 0 => "quadrant 4",
                _ => "axis"
            };
        }
        
        // Positional pattern matching with tuples
        public string ClassifyPoint((int x, int y) point) => point switch
        {
            (0, 0) => "Origin",
            (_, 0) => "On X-axis",
            (0, _) => "On Y-axis",
            _ => "Somewhere else"
        };
        
        // Relational patterns (C# 9.0)
        public string GetGeneration(int birthYear) => birthYear switch
        {
            < 1946 => "Silent Generation",
            >= 1946 and <= 1964 => "Baby Boomer",
            > 1964 and <= 1980 => "Generation X",
            > 1980 and <= 1996 => "Millennial",
            > 1996 and <= 2012 => "Generation Z",
            > 2012 => "Generation Alpha",
            _ => "Unknown"
        };
        
        // List patterns (C# 11)
        public void ListPatternExample(int[] numbers)
        {
            var result = numbers switch
            {
                [] => "Empty",
                [var single] => $"Single: {single}",
                [var first, var second] => $"Pair: {first}, {second}",
                [var first, .., var last] => $"First: {first}, Last: {last}",
                _ => "Multiple elements"
            };
        }
    }
}`

		_, symbols, _ := parser.ParseFile("test.cs", []byte(code))

		// Check for methods
		foundGetPerson := false
		foundSplitName := false
		foundProcessPerson := false
		foundClassifyPoint := false
		foundGetGeneration := false
		foundListPatternExample := false

		for _, sym := range symbols {
			switch sym.Name {
			case "GetPerson":
				foundGetPerson = true
			case "SplitName":
				foundSplitName = true
			case "ProcessPerson":
				foundProcessPerson = true
			case "ClassifyPoint":
				foundClassifyPoint = true
			case "GetGeneration":
				foundGetGeneration = true
			case "ListPatternExample":
				foundListPatternExample = true
			}
		}

		assert.True(t, foundGetPerson, "Should find GetPerson method with tuple return")
		assert.True(t, foundSplitName, "Should find SplitName method with named tuple")
		assert.True(t, foundProcessPerson, "Should find ProcessPerson method")
		assert.True(t, foundClassifyPoint, "Should find ClassifyPoint method")
		assert.True(t, foundGetGeneration, "Should find GetGeneration method with relational patterns")
		assert.True(t, foundListPatternExample, "Should find ListPatternExample method")
	})

	t.Run("parse C# file-scoped namespaces and raw string literals", func(t *testing.T) {
		code := `// File-scoped namespace (C# 10)
namespace MyApp.Models;

using System.Text.Json;

public class ModernFeatures
{
    // Raw string literals (C# 11)
    public string JsonTemplate = """
        {
            "name": "John",
            "age": 30,
            "address": {
                "street": "123 Main St",
                "city": "Anytown"
            }
        }
        """;
    
    // Interpolated raw string literals
    public string GetGreeting(string name) => $"""
        Hello, {name}!
        Welcome to our application.
        Today is {DateTime.Now:yyyy-MM-dd}
        """;
    
    // UTF-8 string literals (C# 11)
    public ReadOnlySpan<byte> Utf8String => "Hello UTF-8"u8;
    
    // Required members (C# 11)
    public required string RequiredProperty { get; init; }
    
    // Generic math (C# 11)
    public static T Add<T>(T left, T right) where T : INumber<T>
    {
        return left + right;
    }
}

// Interface with static abstract members (C# 11)
public interface IOperation<T> where T : IOperation<T>
{
    static abstract T Identity { get; }
    static abstract T Combine(T left, T right);
}`

		_, symbols, _ := parser.ParseFile("test.cs", []byte(code))

		foundModernFeatures := false
		foundJsonTemplate := false
		foundGetGreeting := false
		foundUtf8String := false
		foundRequiredProperty := false
		foundAdd := false
		foundIOperation := false

		for _, sym := range symbols {
			switch sym.Name {
			case "ModernFeatures":
				foundModernFeatures = true
			case "JsonTemplate":
				foundJsonTemplate = true
			case "GetGreeting":
				foundGetGreeting = true
			case "Utf8String":
				foundUtf8String = true
			case "RequiredProperty":
				foundRequiredProperty = true
			case "Add":
				foundAdd = true
			case "IOperation":
				foundIOperation = true
			}
		}

		assert.True(t, foundModernFeatures, "Should find ModernFeatures class")
		assert.True(t, foundJsonTemplate, "Should find JsonTemplate field")
		assert.True(t, foundGetGreeting, "Should find GetGreeting method")
		assert.True(t, foundUtf8String, "Should find Utf8String property")
		assert.True(t, foundRequiredProperty, "Should find RequiredProperty")
		assert.True(t, foundAdd, "Should find Add generic method")
		assert.True(t, foundIOperation, "Should find IOperation interface")
	})
}
