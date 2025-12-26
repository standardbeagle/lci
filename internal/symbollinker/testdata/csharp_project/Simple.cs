using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Threading.Tasks;
using static System.Math;
using MyAlias = System.Collections.Generic.List<string>;

namespace MyApp.Examples
{
    /// <summary>
    /// A simple C# class for testing symbol extraction
    /// </summary>
    public class SimpleClass : INotifyPropertyChanged, IExampleInterface
    {
        private string _privateField;
        protected int _protectedField;
        public string PublicField;
        
        public const string PUBLIC_CONSTANT = "public_value";
        private const string PRIVATE_CONSTANT = "private_value";
        
        public event PropertyChangedEventHandler PropertyChanged;
        public event Action<string> CustomEvent;
        
        static SimpleClass()
        {
            // Static constructor
        }
        
        public SimpleClass()
        {
            _privateField = "initialized";
        }
        
        public SimpleClass(string initialValue) : this()
        {
            _privateField = initialValue;
        }
        
        public string PublicProperty
        {
            get => _privateField;
            set
            {
                if (_privateField != value)
                {
                    _privateField = value;
                    OnPropertyChanged(nameof(PublicProperty));
                }
            }
        }
        
        public int AutoProperty { get; set; }
        
        private string PrivateProperty { get; set; }
        
        protected virtual string ProtectedProperty { get; protected set; }
        
        public string PublicMethod(string param1, int param2 = 10)
        {
            var localVar = "test";
            return PrivateMethod(localVar);
        }
        
        private string PrivateMethod(string input)
        {
            return input.ToUpper();
        }
        
        protected virtual void ProtectedMethod()
        {
            // Protected method implementation
        }
        
        public static string StaticMethod()
        {
            return "static result";
        }
        
        public async Task<string> AsyncMethod()
        {
            await Task.Delay(100);
            return "async result";
        }
        
        public virtual bool VirtualMethod()
        {
            return true;
        }
        
        public abstract bool AbstractMethod();
        
        protected virtual void OnPropertyChanged(string propertyName)
        {
            PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
        }
        
        // Indexer
        public string this[int index]
        {
            get => $"item_{index}";
            set => Console.WriteLine($"Setting {index} to {value}");
        }
        
        // Operator overloading
        public static SimpleClass operator +(SimpleClass a, SimpleClass b)
        {
            return new SimpleClass(a._privateField + b._privateField);
        }
        
        // Finalizer
        ~SimpleClass()
        {
            // Cleanup
        }
    }
    
    public abstract class AbstractBase
    {
        public abstract void MustImplement();
        
        public virtual string VirtualMethod()
        {
            return "base implementation";
        }
    }
    
    public interface IExampleInterface
    {
        string PublicProperty { get; set; }
        bool AbstractMethod();
        void InterfaceMethod();
    }
    
    public interface IGenericInterface<T> where T : class
    {
        T GetItem();
        void SetItem(T item);
    }
    
    public sealed class FinalClass : AbstractBase, IExampleInterface
    {
        public string PublicProperty { get; set; }
        
        public override void MustImplement()
        {
            // Implementation
        }
        
        public bool AbstractMethod()
        {
            return false;
        }
        
        public void InterfaceMethod()
        {
            // Implementation
        }
    }
    
    public struct SimpleStruct
    {
        public int X { get; set; }
        public int Y { get; set; }
        
        public SimpleStruct(int x, int y)
        {
            X = x;
            Y = y;
        }
        
        public double Distance => Sqrt(X * X + Y * Y);
    }
    
    public enum Status
    {
        Pending,
        Active = 1,
        Inactive = 2
    }
    
    public enum Priority : byte
    {
        Low = 1,
        Medium = 2,
        High = 3
    }
    
    public delegate void SimpleDelegate(string message);
    public delegate T GenericDelegate<in T, out R>(T input);
    
    // Records (C# 9+)
    public record PersonRecord(string Name, int Age);
    
    public record class PersonClass(string Name, int Age)
    {
        public string GetDisplayName() => $"{Name} ({Age})";
    }
    
    // Global functions (C# 9+)
    public static class GlobalFunctions
    {
        public static string ProcessString(string input)
        {
            return input.Trim().ToUpper();
        }
        
        public static void GenericMethod<T>(T item) where T : IComparable<T>
        {
            Console.WriteLine(item.ToString());
        }
    }
}

// Namespace without braces (file-scoped namespace - C# 10+)
namespace MyApp.Utilities;

public static class StringExtensions
{
    public static string Reverse(this string input)
    {
        var chars = input.ToCharArray();
        Array.Reverse(chars);
        return new string(chars);
    }
    
    public static bool IsNullOrEmpty(this string input)
    {
        return string.IsNullOrEmpty(input);
    }
}