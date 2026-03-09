using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace MyApp.Services
{
    [Serializable]
    public class UserService : IUserService, IDisposable
    {
        public const string DefaultRole = "user";
        public static readonly int MaxRetries = 3;
        private string _connectionString;

        public UserService(string connectionString)
        {
            _connectionString = connectionString;
        }

        public async Task<User> GetByIdAsync(int id)
        {
            return await Task.FromResult(new User());
        }

        public void Dispose()
        {
            // cleanup
        }

        private void InternalMethod()
        {
            // not exported
        }

        public string Name { get; set; }

        public int Count { get; private set; }

        private int _secret;

        public class NestedConfig
        {
            public string Key { get; set; }
        }
    }

    public interface IUserService
    {
        public Task<User> GetByIdAsync(int id);
    }

    public struct Point
    {
        public double X { get; set; }
        public double Y { get; set; }

        public Point(double x, double y)
        {
            X = x;
            Y = y;
        }

        public double DistanceTo(Point other)
        {
            return Math.Sqrt(Math.Pow(X - other.X, 2) + Math.Pow(Y - other.Y, 2));
        }
    }

    public enum Status
    {
        Active,
        Inactive,
        Suspended
    }

    public record UserRecord(string Name, int Age);

    public record class DetailedRecord(string Id) : UserRecord("", 0)
    {
        public string Description { get; init; }
    }

    public delegate void EventCallback(object sender, EventArgs args);

    public delegate Task<TResult> AsyncFunc<TInput, TResult>(TInput input);

    internal class InternalOnly
    {
        public void ShouldNotAppear() { }
    }

    public abstract class Repository<TEntity> where TEntity : class, new()
    {
        public abstract Task<TEntity> FindAsync(int id);
        public abstract Task<IEnumerable<TEntity>> GetAllAsync();
    }
}
