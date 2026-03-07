#include <iostream>
#include <vector>
#include <algorithm>
#include <string>

// 冒泡排序
void bubbleSort(std::vector<int>& arr) {
    int n = arr.size();
    for (int i = 0; i < n - 1; i++) {
        for (int j = 0; j < n - i - 1; j++) {
            if (arr[j] > arr[j + 1]) {
                std::swap(arr[j], arr[j + 1]);
            }
        }
    }
}

// 快速排序辅助函数
int partition(std::vector<int>& arr, int low, int high) {
    int pivot = arr[high];
    int i = low - 1;
    for (int j = low; j < high; j++) {
        if (arr[j] <= pivot) {
            i++;
            std::swap(arr[i], arr[j]);
        }
    }
    std::swap(arr[i + 1], arr[high]);
    return i + 1;
}

// 快速排序
void quickSort(std::vector<int>& arr, int low, int high) {
    if (low < high) {
        int pi = partition(arr, low, high);
        quickSort(arr, low, pi - 1);
        quickSort(arr, pi + 1, high);
    }
}

// 打印数组
void printArray(const std::vector<int>& arr, const std::string& label) {
    std::cout << label << ": [";
    for (size_t i = 0; i < arr.size(); i++) {
        std::cout << arr[i];
        if (i < arr.size() - 1) std::cout << ", ";
    }
    std::cout << "]" << std::endl;
}

int main() {
    std::vector<int> data1 = {64, 34, 25, 12, 22, 11, 90};
    std::vector<int> data2 = data1;
    std::vector<int> data3 = data1;

    std::cout << "=== C++ 排序算法演示 ===" << std::endl;
    printArray(data1, "原始数据    ");

    bubbleSort(data1);
    printArray(data1, "冒泡排序结果");

    quickSort(data2, 0, (int)data2.size() - 1);
    printArray(data2, "快速排序结果");

    std::sort(data3.begin(), data3.end());
    printArray(data3, "STL 排序结果");

    std::cout << "排序完成！" << std::endl;
    return 0;
}